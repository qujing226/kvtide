package scheduler

import (
	"context"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/block"
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

	batchBudget batchBudget

	longPrefillThreshold uint32
	scheduleRoundDelay   time.Duration
	thresholdSeqs        uint32

	prefillQueueSmall PrefillQueue
	prefillQueueLarge PrefillQueue
	decodeQueue       DecodeQueue

	requestManager  state.RequestStateManager
	executorManager executor.Manager
	blockManager    block.Manager

	patchExecuteChan chan struct{}

	metrics metrics.Metrics
}

func NewScheduler(l *zap.SugaredLogger, cfg *conf.Conf, prefillQS PrefillQueue, prefillQL PrefillQueue, decodeQ DecodeQueue,
	executorManager executor.Manager, requestManager state.RequestStateManager, blockManager block.Manager,
	metrics metrics.Metrics) Scheduler {
	s := &scheduler{
		l:                l,
		patchExecuteChan: make(chan struct{}, 1),

		batchBudget: batchBudget{
			remainTokens:       cfg.Server.ScheduleConf.MaxBatchTokens,
			remainSeqs:         cfg.Server.ScheduleConf.MaxBatchSeq,
			remainPrefill:      cfg.Server.ScheduleConf.MaxPartialPrefills,
			remainLargePrefill: cfg.Server.ScheduleConf.MaxLongPartialPrefills,
		},

		longPrefillThreshold: cfg.Server.ScheduleConf.LongPrefillTokenThreshold,
		scheduleRoundDelay:   cfg.Server.ScheduleConf.ScheduleDelay(),
		thresholdSeqs:        cfg.Server.ScheduleConf.MaxBatchSeq * 4 / 5,

		prefillQueueSmall: prefillQS,
		prefillQueueLarge: prefillQL,
		decodeQueue:       decodeQ,

		executorManager: executorManager,
		blockManager:    blockManager,
		requestManager:  requestManager,

		metrics: metrics,
	}
	return s
}

func (s *scheduler) Enqueue(workItem *model.WorkItem) error {
	workItem.EnqueuedAt = time.Now()
	var err error
	switch workItem.Phase {
	case v1.WorkPhasePrefill:
		if workItem.TokenCntTotal <= s.longPrefillThreshold {
			err = s.prefillQueueSmall.Enqueue(workItem)
		} else {
			err = s.prefillQueueLarge.Enqueue(workItem)
		}
		if err == nil {
			s.trySchedule()
		}
	case v1.WorkPhaseDecode:
		err = s.decodeQueue.Enqueue(workItem)
		if err == nil {
			s.trySchedule()
		}
	default:
		return errors.New(errors.CodeInvalidArgument, "invalid phase for enqueue")
	}

	if err != nil {
		// metrics: injected request
		s.metrics.IncQueueRejected()
		s.requestManager.Fail(workItem.RequestId, err)
		s.l.Errorw("enqueue failed", "phase", workItem.Phase, "error", err)
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
	// assemble work items
	batch := s.pickBatch()
	if len(batch) <= 0 {
		return
	}

	batchLength := len(batch)
	s.trySchedule()

	batchCreateAt := time.Now()

	batchId := utils.MustGenerateUUIDv7()
	err := s.executorManager.Submit(ctx, &model.Batch{
		BatchID:   batchId,
		BatchSize: uint32(batchLength),
		CreateAt:  batchCreateAt,
		Items:     batch,
	})
	if err != nil {
		// Submit err: requeue workItem.
		for _, work := range batch {
			// Rollback blocks.
			s.blockManager.Rollback(work.WorkId)
			s.l.Errorf("failed to submit work: %v", work)
			s.requeueWork(work)
		}
		s.l.Errorf("failed to submit batch: %v, batchId: %s", err, batchId)
		return
	}

	// metrics: observe batch batchSize & infight batch number
	s.observeBatchStatsAndTimeWait(batch, batchCreateAt)
}

func (s *scheduler) pickBatch() []*model.WorkItem {
	budget := s.batchBudget
	batch := make([]*model.WorkItem, 0, budget.remainSeqs)
	s.pickDecode(&batch, &budget)
	s.pickSmallPrefill(&batch, &budget)
	s.pickLargePrefill(&batch, &budget)
	return batch
}

func (s *scheduler) requeueWork(workItems ...*model.WorkItem) {
	for _, work := range workItems {
		work.BlockAllocation = nil
		switch work.Phase {
		case v1.WorkPhaseDecode:
			s.decodeQueue.Requeue(work)
		case v1.WorkPhasePrefill:
			if work.TokenCntTotal <= s.longPrefillThreshold {
				s.prefillQueueSmall.Requeue(work)
			} else {
				s.prefillQueueLarge.Requeue(work)
			}
		}
	}
}

// trySchedule is a trigger for dispatch if queue pressure > threshold.
func (s *scheduler) trySchedule() {
	if s.prefillQueueLarge.Length() > 10 || s.prefillQueueSmall.Length() > 30 ||
		s.decodeQueue.Length() >= s.thresholdSeqs {
		// signal
		select {
		case s.patchExecuteChan <- struct{}{}:
		default:
		}
	}

	// metrics: queueLength
	s.metrics.SetPrefillQueueLength(int(s.prefillQueueSmall.Length() + s.prefillQueueLarge.Length()))
	s.metrics.SetDecodeQueueLength(int(s.decodeQueue.Length()))
}

func (s *scheduler) observeBatchStatsAndTimeWait(batch []*model.WorkItem, now time.Time) {
	prefillItems := 0
	decodeItems := 0

	batchLength := len(batch)

	for _, work := range batch {
		switch work.Phase {
		case v1.WorkPhasePrefill:
			prefillItems++
		case v1.WorkPhaseDecode:
			decodeItems++
		}
	}

	s.metrics.ObserveBatch(batchLength, prefillItems, decodeItems)

	// metrics: observe prefillQueue wait ms
	for i := 0; i < batchLength; i++ {
		s.metrics.ObserveQueueWait(now.Sub(batch[i].EnqueuedAt).Seconds())
	}
}
