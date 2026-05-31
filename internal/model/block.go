package model

// Block stores control-plane metadata for a fixed-size KV cache block.
// The actual K/V tensors live in the executor runtime; this struct only tracks
// ownership, cache identity, and free-list state.
type Block struct {
	ID uint32

	// Hash identifies the token content of this block and its prefix chain.
	Hash string

	// RefCount is the number of active requests or cached entries referencing this block.
	RefCount uint32
	// TokenCount is the number of valid tokens stored in this block.
	TokenCount uint32

	// Cached marks whether this block is indexed by prefix cache metadata.
	Cached      bool
	InFreeQueue bool

	// PrevFree and NextFree use -1 as the empty sentinel.
	// They link free blocks without allocating an extra queue node.
	PrevFree int32
	NextFree int32
}

// BlockAllocation describes the KV block reservation made for one WorkItem.
// The scheduler uses it for admission/rollback; the executor uses its block table
// to simulate runtime KV access.
type BlockAllocation struct {
	RequestID string
	WorkID    string
	BlockSize uint32

	// BlockTable is the full block table visible to this request after allocation.
	BlockTable []uint32
	// AllocatedBlocks are blocks newly reserved for this WorkItem.
	AllocatedBlocks []uint32

	// CachedTokens is the number of prompt tokens reused from prefix cache.
	CachedTokens uint32
	// RequiredTokens is the number of new tokens this WorkItem needs to place in KV cache.
	RequiredTokens uint32
	// RequiredBlocks is the number of additional blocks needed for RequiredTokens.
	RequiredBlocks uint32
	// TokensAfterCommit is the number of tokens after an allocation succeed.
	TokensAfterCommit uint32
}
