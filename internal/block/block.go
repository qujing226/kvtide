package block

import (
	"sync"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/zap"
)

const (
	// TmpBlockSize and TmpTotalBlocks are temporary allocator settings.
	// A real backend should derive them from model config and available KV memory.
	TmpBlockSize   = 16
	TmpTotalBlocks = 1024
)

type Manager interface {
	MatchPrefix(req *model.Request) *model.PrefixMatch
	AllocateBlocks(work *model.WorkItem) bool
	Commit(workID string)
	Rollback(workID string)
	FreeRequest(requestID string)
	Stats() model.BlockStats
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

	mu sync.RWMutex
}

func NewManager(l *zap.SugaredLogger) Manager {
	m := &manager{
		l:                  l,
		blocks:             make([]model.Block, TmpTotalBlocks),
		blockSize:          TmpBlockSize,
		freeHead:           0,
		freeTail:           TmpTotalBlocks - 1,
		freeCount:          TmpTotalBlocks,
		requestBlocks:      make(map[string][]uint32),
		cachedBlocks:       make(map[string]uint32),
		pendingAllocations: make(map[string]*model.BlockAllocation),
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
	return m
}

func (m *manager) MatchPrefix(req *model.Request) *model.PrefixMatch {
	return &model.PrefixMatch{
		RequestID: req.RequestId,
	}
}

func (m *manager) AllocateBlocks(work *model.WorkItem) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentBlocks := uint32(len(m.requestBlocks[work.RequestId]))
	requiredKVToken := requiredKVTokens(work)
	requiredTotalBlocks := ceilDiv(requiredKVToken, m.blockSize)
	requiredBlocks := uint32(0)
	// protect uint32
	if requiredTotalBlocks > currentBlocks {
		requiredBlocks = requiredTotalBlocks - currentBlocks
	}
	if requiredBlocks > m.freeCount {
		return false
	}
	existBlocks := m.requestBlocks[work.RequestId]

	blocks, ok := m.allocate(requiredBlocks)
	if !ok {
		m.l.Errorw("allocate blocks error", "blocks:", requiredBlocks)
		return false
	}

	blockTable := make([]uint32, 0, len(existBlocks)+len(blocks))
	blockTable = append(blockTable, existBlocks...)
	blockTable = append(blockTable, blocks...)
	m.requestBlocks[work.RequestId] = blockTable

	work.BlockAllocation = &model.BlockAllocation{
		RequestID:         work.RequestId,
		WorkID:            work.WorkId,
		BlockSize:         m.blockSize,
		BlockTable:        append([]uint32(nil), blockTable...),
		AllocatedBlocks:   blocks,
		CachedTokens:      work.PrefillOffset,
		RequiredTokens:    work.NumNewTokens,
		RequiredBlocks:    requiredBlocks,
		TokensAfterCommit: requiredKVToken,
	}
	m.pendingAllocations[work.WorkId] = work.BlockAllocation

	return true
}

func (m *manager) Commit(workID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	allocation, exists := m.pendingAllocations[workID]
	if !exists {
		return
	}
	delete(m.pendingAllocations, workID)

	// update tokenCount
	for idx, blockID := range allocation.BlockTable {
		start := uint32(idx) * m.blockSize
		if allocation.TokensAfterCommit <= start {
			m.blocks[blockID].TokenCount = 0
			continue
		}

		remaining := allocation.TokensAfterCommit - start
		if remaining >= m.blockSize {
			m.blocks[blockID].TokenCount = m.blockSize
		} else {
			m.blocks[blockID].TokenCount = remaining
		}
	}
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
}

func (m *manager) Stats() model.BlockStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cachedBlocks := uint64(0)
	for i := range m.blocks {
		if m.blocks[i].Cached {
			cachedBlocks++
		}
	}
	freeBlocks := uint64(m.freeCount)
	totalBlocks := uint64(len(m.blocks))
	return model.BlockStats{
		TotalBlocks:  totalBlocks,
		UsedBlocks:   totalBlocks - freeBlocks,
		FreeBlocks:   freeBlocks,
		CachedBlocks: cachedBlocks,
	}
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

func requiredKVTokens(work *model.WorkItem) uint32 {
	switch work.Phase {
	case v1.WorkPhasePrefill:
		return work.PrefillOffset + work.NumNewTokens
	case v1.WorkPhaseDecode:
		return work.PromptTokens + work.GeneratedTokens + work.NumNewTokens
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
