package scheduler

import (
	"context"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/executor"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"github.com/qujing226/mini-llm-serve/internal/utils"
	"go.uber.org/zap"
)

type Scheduler interface {
	Enqueue(input *model.WorkItem) error
	Batch(ctx context.Context)
}

type scheduler struct {
	l *zap.SugaredLogger

	maxBatchSeqs           uint32
	maxBatchTokens         uint32
	maxPartialPrefills     uint32
	maxLongPartialPrefills uint32
	longPrefillThreshold   uint32
	scheduleRoundDelay     time.Duration
	highPressureSeqs       uint32

	prefillQueueSmall PrefillQueue
	prefillQueueLarge PrefillQueue
	decodeQueue       DecodeQueue

	requestManager  state.RequestLifecycleStateManager
	executorManager executor.Manager

	patchExecuteChan chan struct{}

	metrics metrics.Metrics
}

func NewScheduler(l *zap.SugaredLogger, cfg *conf.Conf, prefillQS PrefillQueue, prefillQL PrefillQueue, decodeQ DecodeQueue, worker executor.Manager, r state.RequestLifecycleStateManager, metrics metrics.Metrics) Scheduler {
	s := &scheduler{
		l:                      l,
		maxBatchSeqs:           cfg.Server.ScheduleConf.MaxBatchSeq,
		maxBatchTokens:         cfg.Server.ScheduleConf.MaxBatchTokens,
		maxPartialPrefills:     cfg.Server.ScheduleConf.MaxPartialPrefills,
		maxLongPartialPrefills: cfg.Server.ScheduleConf.MaxLongPartialPrefills,
		longPrefillThreshold:   cfg.Server.ScheduleConf.LongPrefillTokenThreshold,
		scheduleRoundDelay:     cfg.Server.ScheduleConf.ScheduleDelay(),
		highPressureSeqs:       cfg.Server.ScheduleConf.MaxBatchSeq * 4 / 5,

		prefillQueueSmall: prefillQS,
		prefillQueueLarge: prefillQL,
		decodeQueue:       decodeQ,
		executorManager:   worker,
		requestManager:    r,
		patchExecuteChan:  make(chan struct{}, 1),

		metrics: metrics,
	}
	return s
}

func (s *scheduler) Enqueue(input *model.WorkItem) error {
	input.EnqueuedAt = time.Now()
	var err error
	switch input.Phase {
	case v1.WorkPhasePrefill:
		if input.PromptTokens <= s.longPrefillThreshold {
			err = s.prefillQueueSmall.Enqueue(input)
		} else {
			err = s.prefillQueueLarge.Enqueue(input)
		}
		if err == nil {
			s.trySchedule()
		}
	case v1.WorkPhaseDecode:
		err = s.decodeQueue.Enqueue(input)
		if err == nil {
			s.trySchedule()
		}
	default:
		return errors.New(errors.CodeInvalidArgument, "invalid phase for enqueue")
	}

	if err != nil {
		// metrics: injected request
		s.metrics.IncQueueRejected()
		s.requestManager.Fail(input.RequestId, err)
		s.l.Errorw("enqueue failed", "phase", input.Phase, "error", err)
		return err
	}

	return nil
}

func (s *scheduler) Batch(ctx context.Context) {
	go s.consumeEvents(ctx)

	ticker := time.NewTicker(s.scheduleRoundDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.patchExecuteChan:
			ticker.Reset(s.scheduleRoundDelay)
			s.patchExecute(ctx)
		case <-ticker.C:
			s.patchExecute(ctx)
		}
	}
}

func (s *scheduler) consumeEvents(ctx context.Context) {
	ch := s.executorManager.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			nextItems, err := s.requestManager.OnEvent(event)
			if err != nil {
				s.l.Errorf("failed to execute event: %v", err)
			}
			for _, nextItem := range nextItems {
				_ = s.Enqueue(nextItem)
			}

			s.metrics.ObserveExecution(event.Timing.Execution.Seconds(), event.ExecutorId)
		}
	}
}

func (s *scheduler) patchExecute(ctx context.Context) {
	workItems, taskLength := s.pickBatch()
	if taskLength <= 0 {
		return
	}
	s.trySchedule()

	// metrics: observe batch batchSize & inflight batch number
	s.metrics.ObserveBatchSize(taskLength)

	bid, err := utils.GenerateUUIDv7()
	if err != nil {
		s.l.Errorf("failed to generate batch id: %v", err)
	}
	batchCreateAt := time.Now()
	// metrics: observe prefillQueue wait ms
	for _, task := range workItems {
		s.metrics.ObserveQueueWait(batchCreateAt.Sub(task.EnqueuedAt).Seconds())
	}

	err = s.executorManager.Submit(ctx, &model.Batch{
		BatchID:   bid,
		BatchSize: uint32(len(workItems)),
		CreateAt:  batchCreateAt,
		Items:     workItems,
	})
	if err != nil {
		for _, e := range workItems {
			s.l.Errorf("failed to submit task: %v", e)
			s.requestManager.Fail(e.RequestId, err)
		}
		s.l.Errorf("failed to submit batch: %v, batchId: %s", err, bid)
		return
	}
}

func (s *scheduler) pickBatch() ([]*model.WorkItem, int) {
	remainTokens := s.maxBatchTokens
	remainSeqs := s.maxBatchSeqs
	remainPrefill := s.maxPartialPrefills
	remainLongPrefill := s.maxLongPartialPrefills

	batch := make([]*model.WorkItem, 0, remainSeqs)
	items, itemNums := s.decodeQueue.Dequeue(min(remainSeqs, remainTokens))
	if itemNums != 0 {
		remainTokens -= itemNums
		remainSeqs -= itemNums
		batch = append(batch, items...)
	} else {
		remainPrefill, remainLongPrefill = remainSeqs, remainSeqs
	}

	for remainSeqs > 0 && remainTokens > 0 && remainPrefill > 0 {
		if small, ok := s.prefillQueueSmall.Peek(); ok {
			cost := WorkBudgetCost(small)
			if cost <= remainTokens {
				small, ok = s.prefillQueueSmall.Pop()
				if !ok {
					continue
				}
				batch = append(batch, small)
				remainTokens -= cost
				remainSeqs--
				remainPrefill--
				continue
			}
		}
		if remainLongPrefill <= 0 {
			break
		}
		if large, ok := s.prefillQueueLarge.Peek(); ok {
			large, ok = s.prefillQueueLarge.Pop()
			if !ok {
				continue
			}
			cost := WorkBudgetCost(large)
			scheduledTokens := min(cost, remainTokens)
			chunk, _ := splitPrefillChunk(large, scheduledTokens)

			batch = append(batch, chunk)
			remainTokens -= scheduledTokens
			remainSeqs--
			remainPrefill--
			remainLongPrefill--
			continue
		}
		break
	}

	return batch, len(batch)
}

func (s *scheduler) trySchedule() {
	if s.shouldDispatchNow() {
		s.signalSchedule()
	}
	s.updateQueueMetrics()
}

func (s *scheduler) shouldDispatchNow() bool {
	if s.prefillQueueLarge.Length() > 10 || s.prefillQueueSmall.Length() > 30 {
		return true
	}
	return s.decodeQueue.Length() >= s.highPressureSeqs
}

func (s *scheduler) signalSchedule() {
	select {
	case s.patchExecuteChan <- struct{}{}:
	default:
	}
}

func (s *scheduler) updateQueueMetrics() {
	s.metrics.SetPrefillQueueLength(int(s.prefillQueueSmall.Length() + s.prefillQueueLarge.Length()))
	s.metrics.SetDecodeQueueLength(int(s.decodeQueue.Length()))
}
