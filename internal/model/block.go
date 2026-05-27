package model

type Block struct {
	ID          uint32
	Hash        string
	RefCount    uint32
	TokenCount  uint32
	Cached      bool
	InFreeQueue bool

	// PrevFree and NextFree use -1 as the empty sentinel.
	PrevFree int32
	NextFree int32
}

type BlockAllocation struct {
	RequestID       string
	WorkID          string
	BlockSize       uint32
	BlockTable      []uint32
	AllocatedBlocks []uint32
	CachedTokens    uint64
	RequiredTokens  uint64
	RequiredBlocks  uint64
}
