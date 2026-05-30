package model

// PrefixMatch describes the cached prefix blocks that can be reused by a request.
// It is metadata only; the physical KV tensors are stored by the executor runtime.
type PrefixMatch struct {
	RequestID string
	// CachedTokens is usually len(BlockIDs) * block size, except for partial tail blocks.
	CachedTokens uint32
	BlockIDs     []uint32
	Hit          bool
}
