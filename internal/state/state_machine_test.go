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

func newTestRequestStateManager(m metrics.Metrics) testStateFixture {
	blockManager := block.NewManager(zap.NewNop().Sugar(), metrics.NewMetrics())
	return testStateFixture{
		manager:     NewRequestLifecycleStateManager(zap.NewNop().Sugar(), blockManager, m),
		blockManger: blockManager,
		metrics:     m,
	}
}

func TestOnEventIgnoresStaleEventAfterCancel(t *testing.T) {
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-stale",
		Model:        "mock",
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
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-canceled",
		Model:        "mock",
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
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-timeout",
		Model:        "mock",
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
	fixture := newTestRequestStateManager(m)
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-duplicate",
		Model:        "mock",
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
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-cache-miss",
		Model:        "mock",
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
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	seedPrefixCache(t, fixture, "seed-partial", "shared-prefix", testStateTokenIDs(16))

	req := &model.Request{
		RequestId:    "req-cache-partial-hit",
		Model:        "mock",
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

func TestCreatePrefixCacheFullHitCreatesDecodeWork(t *testing.T) {
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	seedPrefixCache(t, fixture, "seed-full", "shared-prefix", testStateTokenIDs(16))

	req := &model.Request{
		RequestId:    "req-cache-full-hit",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(16),
		PromptTokens: 16,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhaseDecode, work.Phase)
	require.Equal(t, uint32(16), work.PrefillOffset)
	require.Equal(t, uint32(1), work.NumNewTokens)
	require.True(t, work.Cache.Hit)
	require.Equal(t, uint32(16), work.Cache.CachedTokens)
	require.Equal(t, uint32(16), req.ComputedTokens)
}

func TestPrefillFinishedCreatesDecodeWorkAfterBlockCommit(t *testing.T) {
	fixture := newTestRequestStateManager(metrics.NewMetrics())
	manager := fixture.manager
	req := &model.Request{
		RequestId:    "req-cache-store",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(16),
		PromptTokens: 16,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)
	require.True(t, fixture.blockManger.AllocateBlocks(work))
	fixture.blockManger.Commit(work.WorkId)

	next, err := manager.OnEvent(&model.Event{
		WorkId:    work.WorkId,
		RequestId: req.RequestId,
		Type:      v1.EventTypePrefillFinished,
		Usage: model.Usage{
			InputTokens: 16,
		},
	})

	require.NoError(t, err)
	require.Len(t, next, 1)
	require.Equal(t, v1.WorkPhaseDecode, next[0].Phase)

	manager.Finish(req.RequestId)
	followup := &model.Request{
		RequestId:    "req-cache-store-followup",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testStateTokenIDs(16),
		PromptTokens: 16,
	}
	followupWork, err := manager.Create(followup)
	require.NoError(t, err)
	require.Equal(t, v1.WorkPhaseDecode, followupWork.Phase)
	require.True(t, followupWork.Cache.Hit)
	require.Equal(t, uint32(16), followupWork.Cache.CachedTokens)
}

func seedPrefixCache(t *testing.T, fixture testStateFixture, requestId, cacheSalt string, tokens []uint32) {
	t.Helper()

	req := &model.Request{
		RequestId:    requestId,
		Model:        "mock",
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
