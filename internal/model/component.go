package model

import (
	"time"
)

type Batch struct {
	BatchID   string
	BatchSize uint32
	CreateAt  time.Time
	Items     []*WorkItem
}

type Usage struct {
	InputTokens  uint32
	OutputTokens uint32
	TotalTokens  uint32
}

type Timing struct {
	Queue     time.Duration
	BatchWait time.Duration
	Execution time.Duration
	Total     time.Duration
}

// PrefixMatch describes the cached prefix blocks that can be reused by a request.
// It is metadata only; the physical KV tensors are stored by the executor runtime.
type PrefixMatch struct {
	Hit bool
	// CachedTokens is usually len(BlockIDs) * block size, except for partial tail blocks.
	CachedTokens uint32
	// BlockIDs represent cache hit blocks
	BlockIDs []uint32
	// HashesTotal represent all block's hash.
	HashesTotal []string
}

type EngineRuntimeStats struct {
	PrefillQueueLength uint64
	DecodeQueueLength  uint64
	ActiveRequests     uint64
	InflightBatches    uint64
	BusyExecutors      uint64
	IdleExecutors      uint64
}

type BlockStats struct {
	TotalBlocks     uint64
	UsedBlocks      uint64
	FreeBlocks      uint64
	CachedBlocks    uint64
	EvictedBlocks   uint64
	AllocationFails uint64
}

type ExecutorStats struct {
	ExecutorId   string
	RuntimeEpoch uint32

	ModelId              string
	ModelType            string
	Dtype                string
	DeviceType           string
	TensorParallelSize   uint32
	BlockSize            uint32
	NumKvBlocks          uint32
	NumHiddenLayers      uint32
	NumKvHeads           uint32
	HeadDim              uint32
	TotalMemoryBytes     uint64
	AvailableMemoryBytes uint64
	KVCacheBytes         uint64
}
