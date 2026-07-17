package block

import (
	"fmt"
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var testBlockConfig = Config{
	BlockSize: 16,
	NumBlocks: 1024,
}

func newTestManager(t *testing.T) *manager {
	t.Helper()

	m, err := NewManager(zap.NewNop().Sugar(), metrics.NewMetrics(), testBlockConfig)
	require.NoError(t, err)
	return m.(*manager)
}

func TestAllocateBlocksForPrefillUsesTotalKVLength(t *testing.T) {
	m := newTestManager(t)

	work := prefillWork("work-1", "req-1", 0, 40)

	ok := m.AllocateBlocks(work)
	allocation := work.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{0, 1, 2}, allocation.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1, 2}, allocation.BlockTable)
	require.Equal(t, uint32(0), m.blocks[0].TokenCount)
	require.Equal(t, uint32(0), m.blocks[1].TokenCount)
	require.Equal(t, uint32(0), m.blocks[2].TokenCount)

	m.Commit(work.WorkId)
	require.Equal(t, uint32(16), m.blocks[0].TokenCount)
	require.Equal(t, uint32(16), m.blocks[1].TokenCount)
	require.Equal(t, uint32(8), m.blocks[2].TokenCount)

	require.Equal(t, testBlockConfig.NumBlocks-3, m.freeCount)
	require.Equal(t, 2, len(m.cachedBlocks))
}

func TestAllocateBlocksReusesPartiallyFilledPrefillBlock(t *testing.T) {
	m := newTestManager(t)

	firstWork := prefillWork("work-1", "req-1", 0, 8)
	ok := m.AllocateBlocks(firstWork)
	first := firstWork.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{0}, first.AllocatedBlocks)
	require.Equal(t, []uint32{0}, first.BlockTable)
	require.Equal(t, uint32(0), m.blocks[0].TokenCount)

	m.Commit(firstWork.WorkId)
	require.Equal(t, uint32(8), m.blocks[0].TokenCount)

	secondWork := prefillWork("work-2", "req-1", 8, 8)
	ok = m.AllocateBlocks(secondWork)
	second := secondWork.BlockAllocation
	require.True(t, ok)
	require.Empty(t, second.AllocatedBlocks)
	require.Equal(t, []uint32{0}, second.BlockTable)
	require.Equal(t, uint32(8), m.blocks[0].TokenCount)

	m.Commit(secondWork.WorkId)
	require.Equal(t, uint32(16), m.blocks[0].TokenCount)
}

func TestAllocateBlocksOnlyAllocatesWhenDecodeCrossesBlockBoundary(t *testing.T) {
	m := newTestManager(t)

	firstWork := prefillWork("prefill", "req-1", 0, 16)
	ok := m.AllocateBlocks(firstWork)
	first := firstWork.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{0}, first.AllocatedBlocks)
	m.Commit(firstWork.WorkId)

	// Final prefill sampled token 17. Decode computes its KV and crosses into
	// the second physical block.
	withinBlockWork := decodeWork("decode-1", "req-1", 17, 1, 1)
	ok = m.AllocateBlocks(withinBlockWork)
	withinBlock := withinBlockWork.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{1}, withinBlock.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1}, withinBlock.BlockTable)
	m.Commit(withinBlockWork.WorkId)

	stillWithinBlockWork := decodeWork("decode-2", "req-1", 18, 2, 1)
	ok = m.AllocateBlocks(stillWithinBlockWork)
	stillWithinBlock := stillWithinBlockWork.BlockAllocation
	require.True(t, ok)
	require.Empty(t, stillWithinBlock.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1}, stillWithinBlock.BlockTable)
}

func TestFreeRequestReturnsNonCachedBlocksToFreeQueue(t *testing.T) {
	m := newTestManager(t)

	work := prefillWork("work-1", "req-1", 0, 8)
	ok := m.AllocateBlocks(work)
	allocation := work.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{0}, allocation.AllocatedBlocks)

	m.FreeRequest("req-1")
	m.FreeRequest("req-1")

	require.Equal(t, 0, len(m.cachedBlocks))
	require.Equal(t, testBlockConfig.NumBlocks, m.freeCount)
}

func TestPrefixCacheHitRemovesBlocksFromFreeQueueAndCanBeReusedAgain(t *testing.T) {
	m := newTestManager(t)
	tokens := tokenIDs(32)

	firstReq := request("req-1", "user-1", tokens)
	firstMatch := m.MatchPrefix(firstReq)
	require.False(t, firstMatch.Hit)

	firstWork := prefillWorkWithCache("work-1", firstReq, firstMatch, 0, 32)
	require.True(t, m.AllocateBlocks(firstWork))
	m.Commit(firstWork.WorkId)
	m.FreeRequest(firstReq.RequestId)

	require.True(t, m.blocks[0].Cached)
	require.True(t, m.blocks[1].Cached)
	require.True(t, m.blocks[0].InFreeQueue)
	require.True(t, m.blocks[1].InFreeQueue)
	require.Equal(t, uint32(0), m.blocks[0].RefCount)
	require.Equal(t, uint32(0), m.blocks[1].RefCount)
	require.Equal(t, testBlockConfig.NumBlocks, m.freeCount)

	secondReq := request("req-2", "user-1", tokens)
	secondMatch := m.MatchPrefix(secondReq)
	require.True(t, secondMatch.Hit)
	require.Equal(t, uint32(16), secondMatch.CachedTokens)
	require.Equal(t, []uint32{0}, secondMatch.BlockIDs)
	require.False(t, m.blocks[0].InFreeQueue)
	require.True(t, m.blocks[1].InFreeQueue)
	require.Equal(t, uint32(1), m.blocks[0].RefCount)
	require.Equal(t, uint32(0), m.blocks[1].RefCount)
	require.Equal(t, testBlockConfig.NumBlocks-1, m.freeCount)

	m.FreeRequest(secondReq.RequestId)
	require.True(t, m.blocks[0].InFreeQueue)
	require.True(t, m.blocks[1].InFreeQueue)
	require.True(t, m.blocks[0].Cached)
	require.True(t, m.blocks[1].Cached)
	require.Equal(t, testBlockConfig.NumBlocks, m.freeCount)

	thirdReq := request("req-3", "user-1", tokens)
	thirdMatch := m.MatchPrefix(thirdReq)
	require.True(t, thirdMatch.Hit)
	require.Equal(t, []uint32{0}, thirdMatch.BlockIDs)
}

func TestMatchPrefixLeavesFinalLogicalBlockUncached(t *testing.T) {
	tests := []struct {
		promptTokens uint32
		cachedTokens uint32
	}{
		{promptTokens: 15, cachedTokens: 0},
		{promptTokens: 16, cachedTokens: 0},
		{promptTokens: 17, cachedTokens: 16},
		{promptTokens: 32, cachedTokens: 16},
		{promptTokens: 33, cachedTokens: 32},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("prompt_%d", tt.promptTokens), func(t *testing.T) {
			m := newTestManager(t)
			tokens := tokenIDs(tt.promptTokens)

			firstReq := request("req-1", "user-1", tokens)
			firstMatch := m.MatchPrefix(firstReq)
			require.False(t, firstMatch.Hit)
			work := prefillWorkWithCache("work-1", firstReq, firstMatch, 0, tt.promptTokens)
			require.True(t, m.AllocateBlocks(work))
			m.Commit(work.WorkId)
			m.FreeRequest(firstReq.RequestId)

			secondMatch := m.MatchPrefix(request("req-2", "user-1", tokens))
			require.Equal(t, tt.cachedTokens > 0, secondMatch.Hit)
			require.Equal(t, tt.cachedTokens, secondMatch.CachedTokens)
		})
	}
}

func TestPopFreeRemovesStalePrefixCacheIndexWhenOverwritingCachedBlock(t *testing.T) {
	m := newTestManager(t)

	oldHash := "cached-free-block"
	m.blocks[0].Cached = true
	m.blocks[0].Hash = oldHash
	m.blocks[0].TokenCount = testBlockConfig.BlockSize
	m.cachedBlocks[oldHash] = 0
	require.Contains(t, m.cachedBlocks, oldHash)

	id, ok := m.popFree()
	require.True(t, ok)
	require.Equal(t, uint32(0), id)
	require.NotContains(t, m.cachedBlocks, oldHash)
	require.False(t, m.blocks[0].Cached)
	require.Empty(t, m.blocks[0].Hash)
}

func TestMatchPrefixUsesCacheSaltIsolation(t *testing.T) {
	m := newTestManager(t)
	tokens := tokenIDs(17)

	req := request("req-1", "user-1", tokens)
	match := m.MatchPrefix(req)
	work := prefillWorkWithCache("work-1", req, match, 0, 17)
	require.True(t, m.AllocateBlocks(work))
	m.Commit(work.WorkId)
	m.FreeRequest(req.RequestId)

	sameUser := m.MatchPrefix(request("req-2", "user-1", tokens))
	require.True(t, sameUser.Hit)
	m.FreeRequest("req-2")

	otherUser := m.MatchPrefix(request("req-3", "user-2", tokens))
	require.False(t, otherUser.Hit)
}

func prefillWork(workID, requestID string, offset, newTokens uint32) *model.WorkItem {
	return &model.WorkItem{
		WorkId:        workID,
		RequestId:     requestID,
		Phase:         v1.WorkPhasePrefill,
		Cache:         &model.PrefixMatch{HashesTotal: testHashes(ceilDiv(offset+newTokens, testBlockConfig.BlockSize))},
		TokenCntTotal: offset + newTokens,
		PrefillOffset: offset,
		NumNewTokens:  newTokens,
	}
}

func prefillWorkWithCache(workID string, req *model.Request, cache *model.PrefixMatch, offset, newTokens uint32) *model.WorkItem {
	return &model.WorkItem{
		WorkId:        workID,
		RequestId:     req.RequestId,
		Phase:         v1.WorkPhasePrefill,
		Cache:         cache,
		TokenIDs:      req.TokenIDs[offset : offset+newTokens],
		TokenCntTotal: req.PromptTokens,
		PrefillOffset: offset,
		NumNewTokens:  newTokens,
	}
}

func decodeWork(workID, requestID string, inputTokens, generatedTokens, newTokens uint32) *model.WorkItem {
	return &model.WorkItem{
		WorkId:          workID,
		RequestId:       requestID,
		Phase:           v1.WorkPhaseDecode,
		Cache:           &model.PrefixMatch{},
		TokenCntTotal:   inputTokens,
		GeneratedTokens: generatedTokens,
		NumNewTokens:    newTokens,
	}
}

func request(requestID, cacheSalt string, tokens []uint32) *model.Request {
	return &model.Request{
		RequestId:    requestID,
		CacheSalt:    cacheSalt,
		TokenIDs:     tokens,
		PromptTokens: uint32(len(tokens)),
	}
}

func tokenIDs(n uint32) []uint32 {
	tokens := make([]uint32, n)
	for i := range tokens {
		tokens[i] = uint32(i + 1)
	}
	return tokens
}

func testHashes(n uint32) []string {
	hashes := make([]string, n)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("hash-%d", i)
	}
	return hashes
}
