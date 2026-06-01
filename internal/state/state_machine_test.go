package state

import (
	"testing"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/block"
	"github.com/qujing226/mini-llm-serve/internal/cache"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestRequestStateManager(prefixCache cache.PrefixCache, m metrics.Metrics) RequestStateManager {
	return NewRequestLifecycleStateManager(zap.NewNop().Sugar(), prefixCache, block.NewManager(zap.NewNop().Sugar()), m)
}

func TestOnEventIgnoresStaleEventAfterCancel(t *testing.T) {
	manager := newTestRequestStateManager(cache.NewPrefixCache(), metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-stale",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
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
	manager := newTestRequestStateManager(cache.NewPrefixCache(), metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-canceled",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
		PromptTokens: 2,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)

	manager.Cancel(req.RequestId)
	require.False(t, manager.CanSchedule(work))
}

func TestCanScheduleRejectsTimedOutRequest(t *testing.T) {
	manager := newTestRequestStateManager(cache.NewPrefixCache(), metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-timeout",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
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
	manager := newTestRequestStateManager(cache.NewPrefixCache(), m)
	req := &model.Request{
		RequestId:    "req-duplicate",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
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
	prefixCache := cache.NewPrefixCache()
	manager := newTestRequestStateManager(prefixCache, metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-cache-miss",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheKey:     "shared-prefix",
		PromptTokens: 8,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhasePrefill, work.Phase)
	require.Equal(t, uint32(0), work.PrefillOffset)
	require.Equal(t, uint32(8), work.NumNewTokens)
	require.False(t, work.CacheHit)
	require.False(t, req.CacheHit)
	require.Equal(t, uint32(0), req.CachedTokens)
	require.Equal(t, uint32(0), req.ComputedTokens)
}

func TestCreatePrefixCachePartialHitCreatesRemainingPrefillWork(t *testing.T) {
	prefixCache := cache.NewPrefixCache()
	prefixCache.Put("shared-prefix", 5)
	manager := newTestRequestStateManager(prefixCache, metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-cache-partial-hit",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheKey:     "shared-prefix",
		PromptTokens: 8,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhasePrefill, work.Phase)
	require.Equal(t, uint32(5), work.PrefillOffset)
	require.Equal(t, uint32(3), work.NumNewTokens)
	require.True(t, work.CacheHit)
	require.True(t, req.CacheHit)
	require.Equal(t, uint32(5), req.CachedTokens)
	require.Equal(t, uint32(5), req.ComputedTokens)
}

func TestCreatePrefixCacheFullHitCreatesDecodeWork(t *testing.T) {
	prefixCache := cache.NewPrefixCache()
	prefixCache.Put("shared-prefix", 8)
	manager := newTestRequestStateManager(prefixCache, metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-cache-full-hit",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheKey:     "shared-prefix",
		PromptTokens: 8,
	}

	work, err := manager.Create(req)

	require.NoError(t, err)
	require.Equal(t, v1.WorkPhaseDecode, work.Phase)
	require.Equal(t, uint32(8), work.PrefillOffset)
	require.Equal(t, uint32(1), work.NumNewTokens)
	require.True(t, work.CacheHit)
	require.True(t, req.CacheHit)
	require.Equal(t, uint32(8), req.CachedTokens)
	require.Equal(t, uint32(8), req.ComputedTokens)
}

func TestPrefillFinishedStoresPrefixCacheMetadata(t *testing.T) {
	prefixCache := cache.NewPrefixCache()
	manager := newTestRequestStateManager(prefixCache, metrics.NewMetrics())
	req := &model.Request{
		RequestId:    "req-cache-store",
		Model:        "mock",
		Prompt:       "hello world",
		MaxTokens:    8,
		CacheKey:     "shared-prefix",
		PromptTokens: 8,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)

	next, err := manager.OnEvent(&model.Event{
		WorkId:    work.WorkId,
		RequestId: req.RequestId,
		Type:      v1.EventTypePrefillFinished,
		Usage: model.Usage{
			InputTokens: 8,
		},
	})

	require.NoError(t, err)
	require.Len(t, next, 1)
	require.Equal(t, v1.WorkPhaseDecode, next[0].Phase)

	tokens, hit := prefixCache.Lookup("shared-prefix", 8)
	require.True(t, hit)
	require.Equal(t, uint32(8), tokens)
}
