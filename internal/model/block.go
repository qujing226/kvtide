package model

import v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"

// Block stores control-plane metadata for a fixed-size KV cache block.
// The actual K/V tensors live in the executor runtime; this struct only tracks
// ownership, cache identity, and free-list state.
type Block struct {
	ID uint32

	// Hash identifies the token content of this block and its prefix chain.
	Hash string

	// RefCount is the number of active requests referencing this block.
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

	// Phase records the work phase for commit-time cache policy.
	// Prefill can index prompt blocks into prefix cache.
	// Decode only extends the request block table until generated token ids are modeled.
	Phase v1.WorkPhase

	// BlockTable is the full block table visible to this request after allocation.
	BlockTable  []uint32
	BlockHashes []string
	// AllocatedBlocks are blocks newly reserved for this WorkItem.
	AllocatedBlocks []uint32

	// TokensAfterCommit is the number of tokens after an allocation succeed.
	TokensAfterCommit uint32
}
