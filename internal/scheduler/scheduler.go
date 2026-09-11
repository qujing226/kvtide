package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/errors"
	"github.com/qujing226/kvtide/internal/executor"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/qujing226/kvtide/internal/state"
	"github.com/qujing226/kvtide/internal/utils"
	"go.uber.org/zap"
)

type Scheduler interface {
	AssignExecutor(req *model.Request) error
	Enqueue(input *model.WorkItem) error
	Batch(ctx context.Context)
}

type scheduler struct {
	l *zap.SugaredLogger

	batchBudget batchBudget

	longPrefillThreshold uint32
	scheduleRoundDelay   time.Duration
	thresholdSeqs        uint32

	prefillSmall map[string]PrefillQueue
	prefillLarge map[string]PrefillQueue
	decode       map[string]DecodeQueue

	requestManager  state.RequestStateManager
	executorManager executor.Manager
	blockRegistry   block.Registry
	executorIDs     []string
	nextExecutor    atomic.Uint64

	patchExecuteChan chan struct{}

	metrics metrics.Metrics
}

func NewScheduler(l *zap.SugaredLogger, cfg *conf.Conf,
	executorManager executor.Manager, requestManager state.RequestStateManager, blockRegistry block.Registry,
	metrics metrics.Metrics) Scheduler {
	executorIDs := blockRegistry.ExecutorIDs()
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

		prefillSmall: make(map[string]PrefillQueue, len(executorIDs)),
		prefillLarge: make(map[string]PrefillQueue, len(executorIDs)),
		decode:       make(map[string]DecodeQueue, len(executorIDs)),

		executorManager: executorManager,
		blockRegistry:   blockRegistry,
		executorIDs:     executorIDs,
		requestManager:  requestManager,

		metrics: metrics,
	}
	for _, executorID := range executorIDs {
		s.prefillSmall[executorID] = NewPrefillQueue(cfg, requestManager)
		s.prefillLarge[executorID] = NewPrefillQueue(cfg, requestManager)
		s.decode[executorID] = NewDecodeQueue(cfg, requestManager)
	}
	return s
}

func (s *scheduler) AssignExecutor(req *model.Request) error {
	if req.ExecutorID == "" {
		if len(s.executorIDs) == 0 {
			return fmt.Errorf("no executor is available for request %s", req.RequestId)
		}
		idx := s.nextExecutor.Add(1) - 1
		req.ExecutorID = s.executorIDs[idx%uint64(len(s.executorIDs))]
		return nil
	}

	for _, executorID := range s.executorIDs {
		if executorID == req.ExecutorID {
			return nil
		}
	}
	return fmt.Errorf("executor %s is not registered", req.ExecutorID)
}

func (s *scheduler) Enqueue(workItem *model.WorkItem) error {
	workItem.EnqueuedAt = time.Now()
	var err error
	switch workItem.Phase {
	case v1.WorkPhasePrefill:
		var queue PrefillQueue
		var exists bool
		if workItem.TokenCntTotal <= s.longPrefillThreshold {
			queue, exists = s.prefillSmall[workItem.ExecutorID]
		} else {
			queue, exists = s.prefillLarge[workItem.ExecutorID]
		}
		if !exists {
			err = errors.New(errors.CodeExecutorUnavailable, "executor "+workItem.ExecutorID+" has no prefill queue")
			break
		}
		err = queue.Enqueue(workItem)
		if err == nil {
			s.trySchedule(workItem.ExecutorID)
		}
	case v1.WorkPhaseDecode:
		queue, exists := s.decode[workItem.ExecutorID]
		if !exists {
			err = errors.New(errors.CodeExecutorUnavailable, "executor "+workItem.ExecutorID+" has no decode queue")
			break
		}
		err = queue.Enqueue(workItem)
		if err == nil {
			s.trySchedule(workItem.ExecutorID)
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
			s.patchExecutors(ctx)
		case <-ticker.C:
			s.patchExecutors(ctx)
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
			if err := s.handleEvent(event); err != nil {
				s.l.Errorf("failed to handle executor event: %v", err)
			}
		}
	}
}

func (s *scheduler) handleEvent(event *model.Event) error {
	req, exists := s.requestManager.Get(event.RequestId)
	if !exists {
		return nil
	}

	var finalizeErr error
	if event.ExecutorId != req.ExecutorID {
		finalizeErr = fmt.Errorf(
			"event executor %s does not match request executor %s",
			event.ExecutorId,
			req.ExecutorID,
		)
		_ = s.blockRegistry.Rollback(req.ExecutorID, event.WorkId)
	} else if event.Err != nil || event.Type == v1.EventTypeRequestFailed {
		finalizeErr = s.blockRegistry.Rollback(req.ExecutorID, event.WorkId)
	} else {
		finalizeErr = s.blockRegistry.Commit(req.ExecutorID, event.WorkId)
	}

	if finalizeErr != nil {
		event.Err = finalizeErr
		event.Type = v1.EventTypeRequestFailed
		event.Done = true
		event.FinishReason = v1.FinishReasonError
	}

	nextItems, err := s.requestManager.OnEvent(event)
	if err != nil {
		return err
	}
	for _, nextItem := range nextItems {
		if err := s.Enqueue(nextItem); err != nil {
			return err
		}
	}
	if event.Done {
		if err := s.blockRegistry.FreeRequest(req.ExecutorID, req.RequestId); err != nil {
			return err
		}
	}

	if s.metrics != nil {
		s.metrics.ObserveExecution(event.Timing.Execution.Seconds(), event.ExecutorId)
	}
	return finalizeErr
}

func (s *scheduler) patchExecutors(ctx context.Context) {
	for _, executorID := range s.executorIDs {
		s.patchExecute(ctx, executorID)
	}
}

func (s *scheduler) patchExecute(ctx context.Context, executorID string) {
	// assemble work items
	batch := s.pickBatch(executorID)
	if len(batch) <= 0 {
		return
	}

	batchLength := len(batch)
	s.trySchedule(executorID)

	batchCreateAt := time.Now()

	batchId := utils.MustGenerateUUIDv7()
	err := s.executorManager.Submit(ctx, &model.Batch{
		BatchID:    batchId,
		ExecutorID: batch[0].ExecutorID,
		BatchSize:  uint32(batchLength),
		CreateAt:   batchCreateAt,
		Items:      batch,
	})
	if err != nil {
		// Submit err: requeue workItem.
		for _, work := range batch {
			// Rollback blocks.
			if rollbackErr := s.blockRegistry.Rollback(work.ExecutorID, work.WorkId); rollbackErr != nil {
				s.l.Errorw("failed to rollback work", "work", work.WorkId, "error", rollbackErr)
			}
			s.l.Errorf("failed to submit work: %v", work)
			s.requeueWork(work)
		}
		s.l.Errorf("failed to submit batch: %v, batchId: %s", err, batchId)
		return
	}

	// metrics: observe batch batchSize & infight batch number
	s.observeBatchStatsAndTimeWait(batch, batchCreateAt)
}

func (s *scheduler) pickBatch(executorID string) []*model.WorkItem {
	budget := s.batchBudget
	batch := make([]*model.WorkItem, 0, budget.remainSeqs)
	s.pickDecode(s.decode[executorID], &batch, &budget)
	s.pickSmallPrefill(s.prefillSmall[executorID], &batch, &budget)
	s.pickLargePrefill(s.prefillLarge[executorID], &batch, &budget)
	return batch
}

func (s *scheduler) requeueWork(workItems ...*model.WorkItem) {
	for _, work := range workItems {
		work.BlockAllocation = nil
		switch work.Phase {
		case v1.WorkPhaseDecode:
			queue, exists := s.decode[work.ExecutorID]
			if !exists {
				s.requestManager.Fail(work.RequestId, errors.New(errors.CodeExecutorUnavailable, "executor "+work.ExecutorID+" has no decode queue"))
				continue
			}
			queue.Requeue(work)
		case v1.WorkPhasePrefill:
			if work.TokenCntTotal <= s.longPrefillThreshold {
				queue, exists := s.prefillSmall[work.ExecutorID]
				if !exists {
					s.requestManager.Fail(work.RequestId, errors.New(errors.CodeExecutorUnavailable, "executor "+work.ExecutorID+" has no prefill queue"))
					continue
				}
				queue.Requeue(work)
			} else {
				queue, exists := s.prefillLarge[work.ExecutorID]
				if !exists {
					s.requestManager.Fail(work.RequestId, errors.New(errors.CodeExecutorUnavailable, "executor "+work.ExecutorID+" has no prefill queue"))
					continue
				}
				queue.Requeue(work)
			}
		}
	}
}

// trySchedule is a trigger for dispatch if queue pressure > threshold.
func (s *scheduler) trySchedule(executorID string) {
	prefillSmall, smallExists := s.prefillSmall[executorID]
	prefillLarge, largeExists := s.prefillLarge[executorID]
	decode, decodeExists := s.decode[executorID]
	if !smallExists || !largeExists || !decodeExists {
		return
	}
	if prefillLarge.Length() > 10 || prefillSmall.Length() > 30 ||
		decode.Length() >= s.thresholdSeqs {
		// signal
		select {
		case s.patchExecuteChan <- struct{}{}:
		default:
		}
	}

	// metrics: queueLength
	var prefillLength, decodeLength uint32
	for _, id := range s.executorIDs {
		prefillLength += s.prefillSmall[id].Length() + s.prefillLarge[id].Length()
		decodeLength += s.decode[id].Length()
	}
	s.metrics.SetPrefillQueueLength(int(prefillLength))
	s.metrics.SetDecodeQueueLength(int(decodeLength))
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
