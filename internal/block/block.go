package block

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sync"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/zap"
)

type Manager interface {
	MatchPrefix(req *model.Request) *model.PrefixMatch
	AllocateBlocks(work *model.WorkItem) bool
	Commit(workId string)
	Rollback(workID string)
	FreeRequest(requestID string)
}

type manager struct {
	l *zap.SugaredLogger

	blockSize uint32
	blocks    []model.Block

	freeHead  int32
	freeTail  int32
	freeCount uint32

	// requestBlocks maps a request to its logical block table.
	requestBlocks map[string][]uint32
	// cachedBlocks maps a prefix block hash to a reusable block id.
	cachedBlocks map[string]uint32
	// pendingAllocations maps a work.ID to a BlockAllocation which is waiting for commit or rollback.
	pendingAllocations map[string]*model.BlockAllocation

	mu      sync.RWMutex
	metrics metrics.Metrics
}

func NewManager(l *zap.SugaredLogger, metrics metrics.Metrics, cfg Config) (Manager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	m := &manager{
		l:                  l,
		blocks:             make([]model.Block, cfg.NumBlocks),
		blockSize:          cfg.BlockSize,
		freeHead:           0,
		freeTail:           int32(cfg.NumBlocks) - 1,
		freeCount:          cfg.NumBlocks,
		requestBlocks:      make(map[string][]uint32),
		cachedBlocks:       make(map[string]uint32),
		pendingAllocations: make(map[string]*model.BlockAllocation),

		metrics: metrics,
	}
	for i := range m.blocks {
		// Blocks start in a doubly linked free queue.
		m.blocks[i] = model.Block{
			ID:          uint32(i),
			InFreeQueue: true,
			PrevFree:    int32(i - 1),
			NextFree:    int32(i + 1),
		}
	}
	// -1 is the empty sentinel for free-list links.
	m.blocks[0].PrevFree = -1
	m.blocks[len(m.blocks)-1].NextFree = -1

	// set free block
	m.observeBlockStats()
	return m, nil
}

func (m *manager) MatchPrefix(req *model.Request) *model.PrefixMatch {
	var (
		blockIDs []uint32
		hit      bool
	)

	hashes := m.blockHashes(req)

	m.mu.Lock()
	for _, hash := range hashes {
		blockId, exists := m.cachedBlocks[hash]
		if !exists {
			// cached block must be coherent
			break
		}
		blockIDs = append(blockIDs, blockId)
		hit = true

		m.touch(blockId)
	}
	m.requestBlocks[req.RequestId] = append([]uint32(nil), blockIDs...)

	m.observeBlockStats()
	m.mu.Unlock()

	cache := &model.PrefixMatch{
		Hit:          hit,
		CachedTokens: uint32(len(blockIDs)) * m.blockSize,
		BlockIDs:     blockIDs,
		HashesTotal:  hashes,
	}
	req.Cache = cache
	return cache
}

func (m *manager) AllocateBlocks(work *model.WorkItem) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	existBlocks, exist := m.requestBlocks[work.RequestId]
	if !exist {
		if len(work.Cache.BlockIDs) > 0 {
			existBlocks = work.Cache.BlockIDs
			m.requestBlocks[work.RequestId] = existBlocks
		}
	}
	requiredKVToken := requiredKVTokens(work)
	requiredTotalBlocks := ceilDiv(requiredKVToken, m.blockSize)
	requiredBlocks := uint32(0)
	// protect uint32
	if requiredTotalBlocks > uint32(len(existBlocks)) {
		requiredBlocks = requiredTotalBlocks - uint32(len(existBlocks))
	}

	allocatedBlockIds, ok := m.allocate(requiredBlocks)
	if !ok {
		m.metrics.IncAllocationFailure()
		m.l.Errorw("allocate blocks error", "blocks:", requiredBlocks)
		return false
	}

	blockTable := make([]uint32, 0, len(existBlocks)+len(allocatedBlockIds))
	blockTable = append(blockTable, existBlocks...)
	blockTable = append(blockTable, allocatedBlockIds...)
	m.requestBlocks[work.RequestId] = blockTable

	work.BlockAllocation = &model.BlockAllocation{
		RequestID:         work.RequestId,
		WorkID:            work.WorkId,
		Phase:             work.Phase,
		BlockSize:         m.blockSize,
		BlockTable:        append([]uint32(nil), blockTable...),
		BlockHashes:       work.Cache.HashesTotal,
		AllocatedBlocks:   allocatedBlockIds,
		TokensAfterCommit: requiredKVToken,
	}
	m.pendingAllocations[work.WorkId] = work.BlockAllocation
	m.observeBlockStats()
	return true
}

func (m *manager) Commit(workId string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	allocation, exists := m.pendingAllocations[workId]
	if !exists {
		return
	}
	delete(m.pendingAllocations, workId)

	// update tokenCount
	for idx, blockId := range allocation.BlockTable {
		start := uint32(idx) * m.blockSize
		block := &m.blocks[blockId]
		if allocation.TokensAfterCommit <= start {
			block.TokenCount = 0
			continue
		}

		remaining := allocation.TokensAfterCommit - start
		if remaining >= m.blockSize {
			block.TokenCount = m.blockSize
		} else {
			block.TokenCount = remaining
		}
	}

	// update cached block
	for idx, blockID := range allocation.BlockTable {
		block := &m.blocks[blockID]

		if allocation.Phase != v1.WorkPhasePrefill ||
			block.Cached ||
			block.TokenCount != allocation.BlockSize ||
			idx >= len(allocation.BlockHashes) {
			continue
		}

		if _, exists := m.cachedBlocks[allocation.BlockHashes[idx]]; exists {
			continue
		}

		block.Hash = allocation.BlockHashes[idx]
		block.Cached = true
		m.cachedBlocks[block.Hash] = blockID
	}
	m.observeBlockStats()
}

func (m *manager) Rollback(workID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	allocation, exists := m.pendingAllocations[workID]
	if !exists {
		return
	}
	delete(m.pendingAllocations, workID)

	blocks := m.requestBlocks[allocation.RequestID]
	// Allocation is appended to the request block table, so rollback removes
	// only the blocks reserved by this WorkItem and keeps older blocks intact.
	if len(blocks) >= len(allocation.AllocatedBlocks) {
		blocks = blocks[:len(blocks)-len(allocation.AllocatedBlocks)]
	}
	if len(blocks) == 0 {
		delete(m.requestBlocks, allocation.RequestID)
	} else {
		m.requestBlocks[allocation.RequestID] = blocks
	}
	for _, id := range allocation.AllocatedBlocks {
		m.pushFree(id)
	}
	m.observeBlockStats()
}

func (m *manager) FreeRequest(requestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	blocks := m.requestBlocks[requestID]
	delete(m.requestBlocks, requestID)
	for workID, allocation := range m.pendingAllocations {
		if allocation.RequestID == requestID {
			delete(m.pendingAllocations, workID)
		}
	}
	for _, id := range blocks {
		m.pushFree(id)
	}
	m.observeBlockStats()
}

func (m *manager) observeBlockStats() {
	free := uint64(m.freeCount)
	active := uint64(len(m.blocks)) - free
	cached := uint64(len(m.cachedBlocks))
	m.metrics.ObserveBlockStats(active, free, cached)
}

func (m *manager) allocate(n uint32) ([]uint32, bool) {
	if m.freeCount < n {
		return nil, false
	}
	ids := make([]uint32, 0, n)
	for i := uint32(0); i < n; i++ {
		id, ok := m.popFree()
		if !ok {
			for _, rollbackID := range ids {
				m.pushFree(rollbackID)
			}
			return nil, false
		}
		b := &m.blocks[id]
		b.RefCount = 1
		b.InFreeQueue = false
		ids = append(ids, id)
	}
	return ids, true
}

func (m *manager) blockHashes(req *model.Request) []string {
	// don't use m.ceilDiv because we generally won't cache the last block if it is not full.
	fullBlocks := uint32(len(req.TokenIDs)) / m.blockSize
	hashes := make([]string, 0, fullBlocks)
	prev := req.CacheSalt
	for i := uint32(0); i < fullBlocks; i++ {
		start := i * m.blockSize
		end := start + m.blockSize
		curr := hashBlock(prev, req.TokenIDs[start:end])
		prev = curr
		hashes = append(hashes, curr)
	}
	return hashes
}

func (m *manager) touch(blockIds ...uint32) {
	for _, bId := range blockIds {
		b := &m.blocks[bId]
		if b.InFreeQueue {
			m.removeFreeBlock(b)
		}
		b.RefCount++
	}
}

func (m *manager) removeFreeBlock(b *model.Block) {
	if !b.InFreeQueue {
		return
	}

	if b.PrevFree >= 0 {
		m.blocks[b.PrevFree].NextFree = b.NextFree
	} else {
		m.freeHead = b.NextFree
	}

	if b.NextFree >= 0 {
		m.blocks[b.NextFree].PrevFree = b.PrevFree
	} else {
		m.freeTail = b.PrevFree
	}

	b.InFreeQueue = false
	b.NextFree = -1
	b.PrevFree = -1
	m.freeCount--
}

func hashBlock(prevHash string, tokens []uint32) string {
	b := make([]byte, 0, len(prevHash)+len(tokens)*4)
	b = append(b, prevHash...)
	tokenBytes := make([]byte, 4)
	for _, token := range tokens {
		binary.BigEndian.PutUint32(tokenBytes, token)
		b = append(b, tokenBytes...)
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func requiredKVTokens(work *model.WorkItem) uint32 {
	switch work.Phase {
	case v1.WorkPhasePrefill:
		return work.PrefillOffset + work.NumNewTokens
	case v1.WorkPhaseDecode:
		return work.TokenCntTotal + work.GeneratedTokens + work.NumNewTokens
	default:
		return 0
	}
}

// ceilDiv returns ceil(v / divisor) for integer values, equal math.Ceil.
func ceilDiv(v, divisor uint32) uint32 {
	if v == 0 {
		return 0
	}
	return (v + divisor - 1) / divisor
}
