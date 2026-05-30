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
	AllocateSlots(work *model.WorkItem) (*model.BlockAllocation, bool)
	Commit(allocation *model.BlockAllocation)
	Rollback(allocation *model.BlockAllocation)
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

	mu sync.RWMutex
}

func NewManager(l *zap.SugaredLogger) Manager {
	m := &manager{
		l:             l,
		blocks:        make([]model.Block, TmpTotalBlocks),
		blockSize:     TmpBlockSize,
		freeHead:      0,
		freeTail:      TmpTotalBlocks - 1,
		freeCount:     TmpTotalBlocks,
		requestBlocks: make(map[string][]uint32),
		cachedBlocks:  make(map[string]uint32),
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

func (m *manager) AllocateSlots(work *model.WorkItem) (*model.BlockAllocation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentBlocks := uint32(len(m.requestBlocks[work.RequestId]))
	requiredTotalBlocks := ceilDiv(requiredKVTokens(work), m.blockSize)
	requiredBlocks := requiredTotalBlocks - currentBlocks
	if requiredBlocks > m.freeCount {
		return nil, false
	}
	existBlocks := m.requestBlocks[work.RequestId]

	blocks, ok := m.allocate(requiredBlocks)
	if !ok {
		m.l.Errorw("allocate blocks error", "blocks:", requiredBlocks)
		return nil, false
	}

	blockTable := make([]uint32, 0, len(existBlocks)+len(blocks))
	blockTable = append(blockTable, existBlocks...)
	blockTable = append(blockTable, blocks...)
	m.requestBlocks[work.RequestId] = blockTable

	return &model.BlockAllocation{
		RequestID:       work.RequestId,
		WorkID:          work.WorkId,
		BlockSize:       m.blockSize,
		BlockTable:      append([]uint32(nil), blockTable...),
		AllocatedBlocks: blocks,
		CachedTokens:    work.PrefillOffset,
		RequiredTokens:  work.NumNewTokens,
		RequiredBlocks:  requiredBlocks,
	}, true
}

func (m *manager) Commit(allocation *model.BlockAllocation) {
	// MVP: allocation is applied eagerly in AllocateSlots.
}

func (m *manager) Rollback(allocation *model.BlockAllocation) {
	if allocation == nil || len(allocation.AllocatedBlocks) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

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
