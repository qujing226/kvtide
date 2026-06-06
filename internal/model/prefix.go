package model

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
