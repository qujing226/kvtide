package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixCacheLookupMissesForEmptyKey(t *testing.T) {
	prefixCache := NewPrefixCache()

	tokens, hit := prefixCache.Lookup("", 8)

	require.False(t, hit)
	require.Equal(t, uint32(0), tokens)
}

func TestPrefixCacheLookupMissesForUnknownKey(t *testing.T) {
	prefixCache := NewPrefixCache()

	tokens, hit := prefixCache.Lookup("missing", 8)

	require.False(t, hit)
	require.Equal(t, uint32(0), tokens)
}

func TestPrefixCachePutThenLookupHits(t *testing.T) {
	prefixCache := NewPrefixCache()

	prefixCache.Put("shared-prefix", 8)
	tokens, hit := prefixCache.Lookup("shared-prefix", 8)

	require.True(t, hit)
	require.Equal(t, uint32(8), tokens)
}

func TestPrefixCacheLookupCapsCachedTokensAtPromptTokens(t *testing.T) {
	prefixCache := NewPrefixCache()

	prefixCache.Put("shared-prefix", 16)
	tokens, hit := prefixCache.Lookup("shared-prefix", 8)

	require.True(t, hit)
	require.Equal(t, uint32(8), tokens)
}

func TestPrefixCachePutKeepsLargestTokenCount(t *testing.T) {
	prefixCache := NewPrefixCache()

	prefixCache.Put("shared-prefix", 8)
	prefixCache.Put("shared-prefix", 4)
	tokens, hit := prefixCache.Lookup("shared-prefix", 16)

	require.True(t, hit)
	require.Equal(t, uint32(8), tokens)
}

func TestPrefixCachePutIgnoresEmptyKey(t *testing.T) {
	prefixCache := NewPrefixCache()

	prefixCache.Put("", 8)
	tokens, hit := prefixCache.Lookup("", 8)

	require.False(t, hit)
	require.Equal(t, uint32(0), tokens)
}
