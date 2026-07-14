package state

import (
	"testing"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/block"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type testStateFixture struct {
	manager     RequestStateManager
	blockManger block.Manager
	metrics     metrics.Metrics
}

func newTestRequestStateManager(t *testing.T, m metrics.Metrics) testStateFixture {
	t.Helper()

	blockManager, err := block.NewManager(zap.NewNop().Sugar(), metrics.NewMetrics(), block.Config{
		BlockSize: 16,
		NumBlocks: 1024,
	})
	require.NoError(t, err)
	return testStateFixture{
		manager:     NewRequestLifecycleStateManager(zap.NewNop().Sugar(), blockManager, m),
		blockManger: blockManager,
		metrics:     m,
	}
}

func TestOnEventIgnoresStaleEventAfterCancel(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-stale",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testStateTokenIDs(2),
		PromptTokens: 2,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)
	require.NotNil(t, work)

	manager.Cancel(req.RequestId)

	next, err := manager.OnEvent(&model.Event{
		WorkId:    work.WorkId,
		RequestId: req.RequestId,
		Type:      v1.EventTypeDecodeChunk,
		Done:      false,
	})

	require.NoError(t, err)
	require.Nil(t, next)
}

func TestCanScheduleRejectsCanceledRequest(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-canceled",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testStateTokenIDs(2),
		PromptTokens: 2,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)

	manager.Cancel(req.RequestId)
	require.False(t, manager.CanSchedule(work))
}

func TestCanScheduleRejectsTimedOutRequest(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-timeout",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testStateTokenIDs(2),
		PromptTokens: 2,
		Deadline:     time.Now().Add(-time.Second),
	}

	work, err := manager.Create(req)
	require.NoError(t, err)
	ch, err := manager.Subscribe(req.RequestId)
	require.NoError(t, err)

	require.False(t, manager.CanSchedule(work))
	_, ok := manager.Get(req.RequestId)
	require.False(t, ok)

	event, ok := <-ch
	require.True(t, ok)
	require.Equal(t, v1.EventTypeRequestFailed, event.Type)
	require.True(t, event.Done)
	require.Error(t, event.Err)

	_, ok = <-ch
	require.False(t, ok)
}

func TestCreateDuplicateDoesNotIncreaseActiveRequests(t *testing.T) {
	m := metrics.NewMetrics()
	fixture := newTestRequestStateManager(t, m)
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-duplicate",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testStateTokenIDs(2),
		PromptTokens: 2,
	}

	_, err := manager.Create(req)
	require.NoError(t, err)

	_, err = manager.Create(req)
	require.Error(t, err)
	require.Equal(t, uint64(1), m.Snapshot().ActiveRequests)

	manager.Finish(req.RequestId)
	require.Equal(t, uint64(0), m.Snapshot().ActiveRequests)
}

func TestCreatePrefixCacheMissCreatesPrefillWork(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-cache-miss",
		ModelID:      model.MockModel,
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(8),
		PromptTokens: 8,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhasePrefill, work.Phase)
	require.Equal(t, uint32(0), work.PrefillOffset)
	require.Equal(t, uint32(8), work.NumNewTokens)
	require.False(t, work.Cache.Hit)
	require.Equal(t, uint32(0), work.Cache.CachedTokens)
	require.Equal(t, uint32(0), req.ComputedTokens)
}

func TestCreatePrefixCachePartialHitCreatesRemainingPrefillWork(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	seedPrefixCache(t, fixture, "seed-partial", "shared-prefix", testStateTokenIDs(16))

	req := &model.Request{
		RequestId:    "req-cache-partial-hit",
		ModelID:      model.MockModel,
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(24),
		PromptTokens: 24,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhasePrefill, work.Phase)
	require.Equal(t, uint32(16), work.PrefillOffset)
	require.Equal(t, uint32(8), work.NumNewTokens)
	require.True(t, work.Cache.Hit)
	require.Equal(t, uint32(16), work.Cache.CachedTokens)
	require.Equal(t, uint32(16), req.ComputedTokens)
}

func TestCreatePrefixCacheLeavesFinalBlockForPrefill(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	seedPrefixCache(t, fixture, "seed-final-block", "shared-prefix", testStateTokenIDs(17))

	req := &model.Request{
		RequestId:    "req-cache-final-block",
		ModelID:      model.MockModel,
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(17),
		PromptTokens: 17,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhasePrefill, work.Phase)
	require.Equal(t, uint32(16), work.PrefillOffset)
	require.Equal(t, uint32(1), work.NumNewTokens)
	require.True(t, work.Cache.Hit)
	require.Equal(t, uint32(16), work.Cache.CachedTokens)
	require.Equal(t, uint32(16), req.ComputedTokens)
}

func TestPrefillFinishedCreatesDecodeWorkAfterBlockCommit(t *testing.T) {
	fixture := newTestRequestStateManager(t, metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-cache-store",
		ModelID:      model.MockModel,
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(17),
		PromptTokens: 17,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)
	require.True(t, fixture.blockManger.AllocateBlocks(work))
	fixture.blockManger.Commit(work.WorkId)

	next, err := manager.OnEvent(&model.Event{
		WorkId:    work.WorkId,
		RequestId: req.RequestId,
		Type:      v1.EventTypePrefillFinished,
		TokenId:   99,
		Usage: model.Usage{
			InputTokens:  17,
			OutputTokens: 1,
		},
	})

	require.NoError(t, err)
	require.Len(t, next, 1)
	require.Equal(t, v1.WorkPhaseDecode, next[0].Phase)
	require.Equal(t, req.TokenIDs, next[0].TokenIDs)
	require.Equal(t, uint32(len(req.TokenIDs)), next[0].TokenCntTotal)
	require.Equal(t, uint32(1), next[0].GeneratedTokens)
	require.Equal(t, uint32(99), next[0].TokenIDs[len(next[0].TokenIDs)-1])

	manager.Finish(req.RequestId)
	followup := &model.Request{
		RequestId:    "req-cache-store-followup",
		ModelID:      model.MockModel,
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(17),
		PromptTokens: 17,
	}
	followupWork, err := manager.Create(followup)
	require.NoError(t, err)
	require.Equal(t, v1.WorkPhasePrefill, followupWork.Phase)
	require.True(t, followupWork.Cache.Hit)
	require.Equal(t, uint32(16), followupWork.Cache.CachedTokens)
	require.Equal(t, uint32(1), followupWork.NumNewTokens)
}

func seedPrefixCache(t *testing.T, fixture testStateFixture, requestId, cacheSalt string, tokens []uint32) {
	t.Helper()

	req := &model.Request{
		RequestId:    requestId,
		ModelID:      model.MockModel,
		Prompt:       "seed",
		MaxTokens:    8,
		CacheSalt:    cacheSalt,
		TokenIDs:     tokens,
		PromptTokens: uint32(len(tokens)),
	}
	work, err := fixture.manager.Create(req)
	require.NoError(t, err)
	require.True(t, fixture.blockManger.AllocateBlocks(work))
	fixture.blockManger.Commit(work.WorkId)
	fixture.manager.Finish(req.RequestId)
}

func testStateTokenIDs(n uint32) []uint32 {
	tokens := make([]uint32, n)
	for i := range tokens {
		tokens[i] = uint32(i + 1)
	}
	return tokens
}
