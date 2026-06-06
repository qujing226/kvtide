package block

import (
	"fmt"
	"testing"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAllocateBlocksForPrefillUsesTotalKVLength(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)

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

	stats := m.Stats()
	require.Equal(t, uint64(TmpTotalBlocks-3), stats.FreeBlocks)
	require.Equal(t, uint64(3), stats.UsedBlocks)
}

func TestAllocateBlocksReusesPartiallyFilledPrefillBlock(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)

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
	m := NewManager(zap.NewNop().Sugar()).(*manager)

	firstWork := prefillWork("prefill", "req-1", 0, 16)
	ok := m.AllocateBlocks(firstWork)
	first := firstWork.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{0}, first.AllocatedBlocks)
	m.Commit(firstWork.WorkId)

	withinBlockWork := decodeWork("decode-1", "req-1", 16, 0, 1)
	ok = m.AllocateBlocks(withinBlockWork)
	withinBlock := withinBlockWork.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{1}, withinBlock.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1}, withinBlock.BlockTable)
	m.Commit(withinBlockWork.WorkId)

	stillWithinBlockWork := decodeWork("decode-2", "req-1", 16, 1, 1)
	ok = m.AllocateBlocks(stillWithinBlockWork)
	stillWithinBlock := stillWithinBlockWork.BlockAllocation
	require.True(t, ok)
	require.Empty(t, stillWithinBlock.AllocatedBlocks)
	require.Equal(t, []uint32{0, 1}, stillWithinBlock.BlockTable)
}

func TestFreeRequestReturnsNonCachedBlocksToFreeQueue(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)

	work := prefillWork("work-1", "req-1", 0, 8)
	ok := m.AllocateBlocks(work)
	allocation := work.BlockAllocation
	require.True(t, ok)
	require.Equal(t, []uint32{0}, allocation.AllocatedBlocks)

	m.FreeRequest("req-1")
	m.FreeRequest("req-1")

	stats := m.Stats()
	require.Equal(t, uint64(TmpTotalBlocks), stats.FreeBlocks)
	require.Equal(t, uint64(0), stats.UsedBlocks)
}

func TestPrefixCacheHitRemovesBlocksFromFreeQueueAndCanBeReusedAgain(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)
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
	require.Equal(t, uint32(TmpTotalBlocks), m.freeCount)

	secondReq := request("req-2", "user-1", tokens)
	secondMatch := m.MatchPrefix(secondReq)
	require.True(t, secondMatch.Hit)
	require.Equal(t, uint32(32), secondMatch.CachedTokens)
	require.Equal(t, []uint32{0, 1}, secondMatch.BlockIDs)
	require.False(t, m.blocks[0].InFreeQueue)
	require.False(t, m.blocks[1].InFreeQueue)
	require.Equal(t, uint32(1), m.blocks[0].RefCount)
	require.Equal(t, uint32(1), m.blocks[1].RefCount)
	require.Equal(t, uint32(TmpTotalBlocks-2), m.freeCount)

	m.FreeRequest(secondReq.RequestId)
	require.True(t, m.blocks[0].InFreeQueue)
	require.True(t, m.blocks[1].InFreeQueue)
	require.True(t, m.blocks[0].Cached)
	require.True(t, m.blocks[1].Cached)
	require.Equal(t, uint32(TmpTotalBlocks), m.freeCount)

	thirdReq := request("req-3", "user-1", tokens)
	thirdMatch := m.MatchPrefix(thirdReq)
	require.True(t, thirdMatch.Hit)
	require.Equal(t, []uint32{0, 1}, thirdMatch.BlockIDs)
}

func TestPopFreeRemovesStalePrefixCacheIndexWhenOverwritingCachedBlock(t *testing.T) {
	m := NewManager(zap.NewNop().Sugar()).(*manager)

	oldHash := "cached-free-block"
	m.blocks[0].Cached = true
	m.blocks[0].Hash = oldHash
	m.blocks[0].TokenCount = TmpBlockSize
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
	m := NewManager(zap.NewNop().Sugar()).(*manager)
	tokens := tokenIDs(16)

	req := request("req-1", "user-1", tokens)
	match := m.MatchPrefix(req)
	work := prefillWorkWithCache("work-1", req, match, 0, 16)
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
		Cache:         &model.PrefixMatch{HashesTotal: testHashes(ceilDiv(offset+newTokens, TmpBlockSize))},
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
