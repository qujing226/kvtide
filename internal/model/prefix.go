package model

type PrefixMatch struct {
	RequestID    string
	CachedTokens uint64
	BlockIDs     []uint32
	Hit          bool
}
