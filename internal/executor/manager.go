package executor

import (
	"context"
	"sort"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/errors"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"go.uber.org/zap"
)

type Manager interface {
	Consume(ctx context.Context)
	Submit(ctx context.Context, batch *model.Batch) error
	TriggerKVPush(ctx context.Context, request *v1.TriggerKVPushRequest) (*v1.TriggerKVPushResponse, error)
	Events() <-chan *model.Event
	GetRuntimeStates() map[string]*model.ExecutorStats
}

type executorManager struct {
	logger *zap.SugaredLogger

	executors    map[string]Executor
	executorList []string

	batchChans map[string]chan *model.Batch
	eventChan  chan *model.Event

	metrics         metrics.Metrics
	inflightBatches atomic.Uint64
}

func (e *executorManager) GetRuntimeStates() map[string]*model.ExecutorStats {
	runtimeStats := make(map[string]*model.ExecutorStats)
	for _, executor := range e.executors {
		runtimeStates := executor.GetRuntimeStates()
		runtimeStats[runtimeStates.ExecutorId] = runtimeStates
	}
	return runtimeStats
}

func NewExecutorManager(logger *zap.SugaredLogger, executors map[string]Executor, metrics metrics.Metrics) Manager {
	executorNum := len(executors)

	executorList := make([]string, 0, executorNum)
	batchChans := make(map[string]chan *model.Batch, executorNum)
	for executorID := range executors {
		executorList = append(executorList, executorID)
		batchChans[executorID] = make(chan *model.Batch, 100)
	}
	sort.Strings(executorList)

	e := &executorManager{
		logger:       logger,
		executors:    executors,
		executorList: executorList,
		batchChans:   batchChans,
		eventChan:    make(chan *model.Event, 100),
		metrics:      metrics,
	}
	return e
}

func (e *executorManager) Submit(ctx context.Context, batch *model.Batch) error {
	if batch == nil {
		return errors.New(errors.CodeInvalidArgument, "batch must not be nil")
	}
	batchChan, exists := e.batchChans[batch.ExecutorID]
	if !exists {
		return errors.New(
			errors.CodeExecutorUnavailable,
			"executor "+batch.ExecutorID+" is unavailable",
		)
	}
	select {
	case batchChan <- batch:
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New(errors.CodeQueueFull, "executor batch queue is full")
	}
	return nil
}

func (e *executorManager) Events() <-chan *model.Event {
	return e.eventChan
}

func (e *executorManager) TriggerKVPush(
	ctx context.Context,
	request *v1.TriggerKVPushRequest,
) (*v1.TriggerKVPushResponse, error) {
	source, ok := e.executors[request.GetSourceExecutorId()]
	if !ok {
		return nil, errors.New(
			errors.CodeExecutorUnavailable,
			"source executor "+request.GetSourceExecutorId()+" is unavailable",
		)
	}
	return source.TriggerKVPush(ctx, request)
}

func (e *executorManager) Consume(ctx context.Context) {
	for _, executorId := range e.executorList {
		go e.consumeExecutor(
			ctx,
			executorId,
			e.executors[executorId],
			e.batchChans[executorId],
		)
	}
	<-ctx.Done()
}

func (e *executorManager) consumeExecutor(
	ctx context.Context,
	executorId string,
	executor Executor,
	batchChan <-chan *model.Batch,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-batchChan:
			if !ok {
				return
			}

			e.metrics.SetInflightBatches(int(e.inflightBatches.Add(1)))
			e.metrics.IncBatches(executorId)
			events, err := executor.Execute(ctx, batch)
			if err != nil {
				// envents.Lenth == 0: execute error.
				if len(events) == 0 {
					events = batchFailedEvents(batch, executorId, err)
				} else {
					markEventsFailed(events, err)
				}
				e.metrics.IncExecutorErrors(executorId)
			}
			e.metrics.SetInflightBatches(int(e.inflightBatches.Add(^uint64(0))))

			for _, event := range events {
				select {
				case e.eventChan <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func markEventsFailed(events []*model.Event, err error) {
	for _, event := range events {
		event.Err = err
		event.Done = true
		event.FinishReason = v1.FinishReasonError
		event.Type = v1.EventTypeRequestFailed
	}
}

func batchFailedEvents(batch *model.Batch, executorId string, err error) []*model.Event {
	events := make([]*model.Event, 0, len(batch.Items))
	for _, item := range batch.Items {
		events = append(events, &model.Event{
			WorkId:       item.WorkId,
			RequestId:    item.RequestId,
			BatchId:      batch.BatchID,
			ExecutorId:   executorId,
			Type:         v1.EventTypeRequestFailed,
			Done:         true,
			FinishReason: v1.FinishReasonError,
			At:           time.Now(),
			Err:          err,
		})
	}
	return events
}
