package scheduler

import (
	"testing"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/qujing226/kvtide/internal/state"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestBlockRegistry(t *testing.T, executorIDs ...string) block.Registry {
	t.Helper()
	configs := make([]block.Config, 0, len(executorIDs))
	for _, executorID := range executorIDs {
		configs = append(configs, block.Config{
			ExecutorID: executorID,
			BlockSize:  16,
			NumBlocks:  1024,
		})
	}
	registry, err := block.NewRegistry(zap.NewNop().Sugar(), metrics.NewMetrics(), configs)
	require.NoError(t, err)
	return registry
}

func newTestScheduler() *scheduler {
	cfg, manager := testQueueConf(16)
	registry := newTestBlockRegistryForScheduler("executor-a")
	return &scheduler{
		batchBudget: batchBudget{
			remainTokens:       16,
			remainSeqs:         8,
			remainPrefill:      2,
			remainLargePrefill: 1,
		},
		longPrefillThreshold: 8,
		prefillSmall: map[string]PrefillQueue{
			"executor-a": NewPrefillQueue(cfg, manager),
		},
		prefillLarge: map[string]PrefillQueue{
			"executor-a": NewPrefillQueue(cfg, manager),
		},
		decode: map[string]DecodeQueue{
			"executor-a": NewDecodeQueue(cfg, manager),
		},
		requestManager: manager,
		blockRegistry:  registry,
		executorIDs:    registry.ExecutorIDs(),
		metrics:        metrics.NewMetrics(),
	}
}

func newTestBlockRegistryForScheduler(executorIDs ...string) block.Registry {
	configs := make([]block.Config, 0, len(executorIDs))
	for _, executorID := range executorIDs {
		configs = append(configs, block.Config{
			ExecutorID: executorID,
			BlockSize:  16,
			NumBlocks:  1024,
		})
	}
	registry, err := block.NewRegistry(zap.NewNop().Sugar(), metrics.NewMetrics(), configs)
	if err != nil {
		panic(err)
	}
	return registry
}

func testSchedulerWork(t *testing.T, s *scheduler, id string, phase v1.WorkPhase, promptTokens uint32, prefillTokens uint32) *model.WorkItem {
	t.Helper()

	work, err := s.requestManager.Create(&model.Request{
		RequestId:    "req-" + id,
		ExecutorID:   "executor-a",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testTokenIDs(promptTokens),
		PromptTokens: promptTokens,
		Deadline:     time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	work.WorkId = id
	work.Phase = phase
	work.TokenCntTotal = promptTokens
	work.NumNewTokens = prefillTokens
	return work
}

func testTokenIDs(n uint32) []uint32 {
	tokens := make([]uint32, n)
	for i := range tokens {
		tokens[i] = uint32(i + 1)
	}
	return tokens
}

func requirePickBatchReturns(t *testing.T, s *scheduler) ([]*model.WorkItem, int) {
	t.Helper()

	type result struct {
		items []*model.WorkItem
		n     int
	}
	ch := make(chan result, 1)
	go func() {
		items := s.pickBatch("executor-a")
		ch <- result{items: items, n: len(items)}
	}()

	select {
	case got := <-ch:
		return got.items, got.n
	case <-time.After(200 * time.Millisecond):
		t.Fatal("pickBatch did not return")
		return nil, 0
	}
}

func TestPickBatchSchedulesDecodeBeforePrefill(t *testing.T) {
	s := newTestScheduler()

	require.NoError(t, s.decode["executor-a"].Enqueue(testSchedulerWork(t, s, "d1", v1.WorkPhaseDecode, 2, 0)))
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p1", v1.WorkPhasePrefill, 4, 4)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "p1", items[1].WorkId)
	require.Equal(t, uint32(0), s.decode["executor-a"].Length())
	require.Equal(t, uint32(0), s.prefillSmall["executor-a"].Length())
}

func TestPickBatchContainsOnlyOneExecutor(t *testing.T) {
	s := newTestScheduler()
	s.blockRegistry = newTestBlockRegistry(t, "executor-a", "executor-b")
	s.executorIDs = s.blockRegistry.ExecutorIDs()
	cfg, _ := testQueueConf(16)
	s.prefillSmall["executor-b"] = NewPrefillQueue(cfg, s.requestManager)
	s.prefillLarge["executor-b"] = NewPrefillQueue(cfg, s.requestManager)
	s.decode["executor-b"] = NewDecodeQueue(cfg, s.requestManager)

	decode := testSchedulerWork(t, s, "d1", v1.WorkPhaseDecode, 2, 0)
	decode.ExecutorID = "executor-a"
	prefill := testSchedulerWork(t, s, "p1", v1.WorkPhasePrefill, 4, 4)
	prefill.ExecutorID = "executor-b"
	require.NoError(t, s.decode["executor-a"].Enqueue(decode))
	require.NoError(t, s.prefillSmall["executor-b"].Enqueue(prefill))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 1, n)
	require.Equal(t, "executor-a", items[0].ExecutorID)
	require.Equal(t, uint32(1), s.prefillSmall["executor-b"].Length())

	items = s.pickBatch("executor-b")
	require.Len(t, items, 1)
	require.Equal(t, "executor-b", items[0].ExecutorID)
	require.Equal(t, uint32(0), s.prefillSmall["executor-b"].Length())
}

func TestEnqueueRoutesWorkToExecutorQueue(t *testing.T) {
	s := newTestScheduler()
	cfg, manager := testQueueConf(16)
	s.prefillSmall["executor-b"] = NewPrefillQueue(cfg, manager)
	s.prefillLarge["executor-b"] = NewPrefillQueue(cfg, manager)
	s.decode["executor-b"] = NewDecodeQueue(cfg, manager)

	work := &model.WorkItem{
		WorkId:        "decode-b",
		RequestId:     "request-b",
		ExecutorID:    "executor-b",
		Phase:         v1.WorkPhaseDecode,
		TokenCntTotal: 2,
	}
	require.NoError(t, s.Enqueue(work))

	require.Equal(t, uint32(0), s.decode["executor-a"].Length())
	require.Equal(t, uint32(1), s.decode["executor-b"].Length())
}

func TestAssignExecutorUsesStableRoundRobin(t *testing.T) {
	registry := newTestBlockRegistry(t, "executor-b", "executor-a")
	s := &scheduler{
		blockRegistry: registry,
		executorIDs:   registry.ExecutorIDs(),
	}

	first := &model.Request{
		RequestId: "request-1",
		CacheSalt: "user-1",
		TokenIDs:  testTokenIDs(2),
	}
	second := &model.Request{
		RequestId: "request-2",
		CacheSalt: "user-2",
		TokenIDs:  testTokenIDs(2),
	}

	require.NoError(t, s.AssignExecutor(first))
	require.NoError(t, s.AssignExecutor(second))
	require.Equal(t, "executor-a", first.ExecutorID)
	require.Equal(t, "executor-b", second.ExecutorID)
	require.Nil(t, first.Cache)
	require.Nil(t, second.Cache)
}

func TestAssignExecutorRejectsUnknownPinnedExecutor(t *testing.T) {
	registry := newTestBlockRegistry(t, "executor-a")
	s := &scheduler{
		blockRegistry: registry,
		executorIDs:   registry.ExecutorIDs(),
	}
	req := &model.Request{
		RequestId:  "request-1",
		ExecutorID: "missing",
		CacheSalt:  "user-1",
		TokenIDs:   testTokenIDs(2),
	}

	require.Error(t, s.AssignExecutor(req))
}

func TestHandleEventCommitsBlocksOnBoundExecutor(t *testing.T) {
	registry := newTestBlockRegistry(t, "executor-a")
	m := metrics.NewMetrics()
	requestManager := state.NewRequestLifecycleStateManager(
		zap.NewNop().Sugar(),
		registry,
		m,
	)
	cfg := &conf.Conf{Server: conf.ServerConf{ScheduleConf: conf.ScheduleConf{
		QueueConf: conf.QueueConf{QueueLength: 16},
	}}}
	s := &scheduler{
		l:                    zap.NewNop().Sugar(),
		longPrefillThreshold: 512,
		prefillSmall: map[string]PrefillQueue{
			"executor-a": NewPrefillQueue(cfg, requestManager),
		},
		prefillLarge: map[string]PrefillQueue{
			"executor-a": NewPrefillQueue(cfg, requestManager),
		},
		decode: map[string]DecodeQueue{
			"executor-a": NewDecodeQueue(cfg, requestManager),
		},
		requestManager: requestManager,
		blockRegistry:  registry,
		executorIDs:    registry.ExecutorIDs(),
		metrics:        m,
	}
	req := &model.Request{
		RequestId:    "request-1",
		ExecutorID:   "executor-a",
		ModelID:      model.MockModel,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testTokenIDs(17),
		PromptTokens: 17,
		MaxTokens:    8,
	}
	require.NoError(t, s.AssignExecutor(req))
	work, err := requestManager.Create(req)
	require.NoError(t, err)
	allocated, err := s.blockRegistry.AllocateBlocks(work)
	require.NoError(t, err)
	require.True(t, allocated)

	require.NoError(t, s.handleEvent(&model.Event{
		WorkId:     work.WorkId,
		RequestId:  req.RequestId,
		ExecutorId: "executor-a",
		Type:       v1.EventTypePrefillFinished,
		TokenId:    99,
		Usage: model.Usage{
			InputTokens:  17,
			OutputTokens: 1,
		},
	}))
	requestManager.Finish(req.RequestId)

	followup := &model.Request{
		RequestId:    "request-2",
		ExecutorID:   "executor-a",
		CacheSalt:    "shared-prefix",
		TokenIDs:     testTokenIDs(17),
		PromptTokens: 17,
	}
	require.NoError(t, s.AssignExecutor(followup))
	_, err = requestManager.Create(followup)
	require.NoError(t, err)
	require.True(t, followup.Cache.Hit)
	require.Equal(t, uint32(16), followup.Cache.CachedTokens)
}

func TestHandleEventRejectsDifferentExecutor(t *testing.T) {
	registry := newTestBlockRegistry(t, "executor-a", "executor-b")
	m := metrics.NewMetrics()
	requestManager := state.NewRequestLifecycleStateManager(
		zap.NewNop().Sugar(),
		registry,
		m,
	)
	s := &scheduler{
		l:              zap.NewNop().Sugar(),
		requestManager: requestManager,
		blockRegistry:  registry,
		executorIDs:    registry.ExecutorIDs(),
		metrics:        m,
	}
	req := &model.Request{
		RequestId:    "request-1",
		ExecutorID:   "executor-a",
		ModelID:      model.MockModel,
		CacheSalt:    "shared-prefix",
		TokenIDs:     testTokenIDs(2),
		PromptTokens: 2,
		MaxTokens:    8,
	}
	require.NoError(t, s.AssignExecutor(req))
	work, err := requestManager.Create(req)
	require.NoError(t, err)
	allocated, err := s.blockRegistry.AllocateBlocks(work)
	require.NoError(t, err)
	require.True(t, allocated)

	event := &model.Event{
		WorkId:     work.WorkId,
		RequestId:  req.RequestId,
		ExecutorId: "executor-b",
		Type:       v1.EventTypePrefillFinished,
	}
	err = s.handleEvent(event)

	require.Error(t, err)
	require.Equal(t, v1.EventTypeRequestFailed, event.Type)
	require.True(t, event.Done)
	require.Error(t, event.Err)
}

func TestPickBatchUsesRemainingTokenBudgetForSmallPrefill(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 6
	s.batchBudget.remainSeqs = 4

	require.NoError(t, s.decode["executor-a"].Enqueue(testSchedulerWork(t, s, "d1", v1.WorkPhaseDecode, 2, 0)))
	require.NoError(t, s.decode["executor-a"].Enqueue(testSchedulerWork(t, s, "d2", v1.WorkPhaseDecode, 2, 0)))
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p1", v1.WorkPhasePrefill, 4, 4)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 3, n)
	require.Len(t, items, 3)
	require.Equal(t, []string{"d1", "d2", "p1"}, []string{
		items[0].WorkId,
		items[1].WorkId,
		items[2].WorkId,
	})
}

func TestPickBatchDoesNotScheduleSmallPrefillThatExceedsRemainingBudget(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 5
	s.batchBudget.remainSeqs = 4

	require.NoError(t, s.decode["executor-a"].Enqueue(testSchedulerWork(t, s, "d1", v1.WorkPhaseDecode, 2, 0)))
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p1", v1.WorkPhasePrefill, 8, 8)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 1, n)
	require.Len(t, items, 1)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, uint32(1), s.prefillSmall["executor-a"].Length())
}

func TestPickBatchChunksLargePrefillWithoutRequeueingRemainder(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 10
	s.batchBudget.remainSeqs = 2
	s.batchBudget.remainPrefill = 1
	s.batchBudget.remainLargePrefill = 1

	require.NoError(t, s.prefillLarge["executor-a"].Enqueue(testSchedulerWork(t, s, "large", v1.WorkPhasePrefill, 24, 24)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 1, n)
	require.Len(t, items, 1)
	require.Equal(t, "large", items[0].WorkId)
	require.Equal(t, uint32(0), items[0].PrefillOffset)
	require.Equal(t, uint32(10), items[0].NumNewTokens)
	require.Equal(t, uint32(0), s.prefillLarge["executor-a"].Length())
}

func TestPickBatchFillsPrefillOnlyBatchBeyondPartialLimit(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 16
	s.batchBudget.remainSeqs = 8
	s.batchBudget.remainPrefill = 1

	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p1", v1.WorkPhasePrefill, 2, 2)))
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p2", v1.WorkPhasePrefill, 2, 2)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "p1", items[0].WorkId)
	require.Equal(t, "p2", items[1].WorkId)
	require.Equal(t, uint32(0), s.prefillSmall["executor-a"].Length())
}

func TestPickBatchRespectsMaxPartialPrefillsWhenDecodeIsPresent(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 16
	s.batchBudget.remainSeqs = 8
	s.batchBudget.remainPrefill = 1

	require.NoError(t, s.decode["executor-a"].Enqueue(testSchedulerWork(t, s, "d1", v1.WorkPhaseDecode, 2, 0)))
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p1", v1.WorkPhasePrefill, 2, 2)))
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(testSchedulerWork(t, s, "p2", v1.WorkPhasePrefill, 2, 2)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "p1", items[1].WorkId)
	require.Equal(t, uint32(1), s.prefillSmall["executor-a"].Length())
}

func TestPickBatchFillsPrefillOnlyBatchBeyondLongPartialLimit(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 16
	s.batchBudget.remainSeqs = 8
	s.batchBudget.remainPrefill = 2
	s.batchBudget.remainLargePrefill = 1

	require.NoError(t, s.prefillLarge["executor-a"].Enqueue(testSchedulerWork(t, s, "l1", v1.WorkPhasePrefill, 4, 4)))
	require.NoError(t, s.prefillLarge["executor-a"].Enqueue(testSchedulerWork(t, s, "l2", v1.WorkPhasePrefill, 4, 4)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "l1", items[0].WorkId)
	require.Equal(t, "l2", items[1].WorkId)
	require.Equal(t, uint32(0), s.prefillLarge["executor-a"].Length())
}

func TestPickBatchRespectsMaxLongPartialPrefillsWhenDecodeIsPresent(t *testing.T) {
	s := newTestScheduler()
	s.batchBudget.remainTokens = 16
	s.batchBudget.remainSeqs = 8
	s.batchBudget.remainPrefill = 2
	s.batchBudget.remainLargePrefill = 1

	require.NoError(t, s.decode["executor-a"].Enqueue(testSchedulerWork(t, s, "d1", v1.WorkPhaseDecode, 2, 0)))
	require.NoError(t, s.prefillLarge["executor-a"].Enqueue(testSchedulerWork(t, s, "l1", v1.WorkPhasePrefill, 4, 4)))
	require.NoError(t, s.prefillLarge["executor-a"].Enqueue(testSchedulerWork(t, s, "l2", v1.WorkPhasePrefill, 4, 4)))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "l1", items[1].WorkId)
	require.Equal(t, uint32(1), s.prefillLarge["executor-a"].Length())
}

func TestPickBatchDropsCanceledWork(t *testing.T) {
	s := newTestScheduler()
	req := &model.Request{
		RequestId:    "req-canceled",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testTokenIDs(2),
		PromptTokens: 2,
	}

	work, err := s.requestManager.Create(req)
	require.NoError(t, err)
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(work))

	s.requestManager.Cancel(req.RequestId)

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 0, n)
	require.Empty(t, items)
	require.Equal(t, uint32(0), s.prefillSmall["executor-a"].Length())
}

func TestPickBatchDropsTimedOutWork(t *testing.T) {
	s := newTestScheduler()
	req := &model.Request{
		RequestId:    "req-timeout",
		ModelID:      model.MockModel,
		Prompt:       "hello",
		MaxTokens:    8,
		TokenIDs:     testTokenIDs(2),
		PromptTokens: 2,
		Deadline:     time.Now().Add(-time.Second),
	}

	work, err := s.requestManager.Create(req)
	require.NoError(t, err)
	require.NoError(t, s.prefillSmall["executor-a"].Enqueue(work))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 0, n)
	require.Empty(t, items)
	require.Equal(t, uint32(0), s.prefillSmall["executor-a"].Length())
}
