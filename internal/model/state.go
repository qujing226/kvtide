package model

type RuntimeStats struct {
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
