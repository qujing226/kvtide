package executor

import (
	"context"
	"testing"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type recordingExecutor struct {
	id      string
	batches chan *model.Batch
}

func (e *recordingExecutor) Execute(_ context.Context, batch *model.Batch) ([]*model.Event, error) {
	e.batches <- batch
	return nil, nil
}

func (e *recordingExecutor) TriggerKVPush(
	context.Context,
	*v1.TriggerKVPushRequest,
) (*v1.TriggerKVPushResponse, error) {
	return nil, nil
}

func (e *recordingExecutor) GetRuntimeStates() *model.ExecutorStats {
	return &model.ExecutorStats{ExecutorId: e.id}
}

func TestManagerDispatchesBatchOnlyToItsExecutor(t *testing.T) {
	source := &recordingExecutor{id: "source", batches: make(chan *model.Batch, 1)}
	destination := &recordingExecutor{id: "destination", batches: make(chan *model.Batch, 1)}
	manager := NewExecutorManager(
		zap.NewNop().Sugar(),
		map[string]Executor{
			"source":      source,
			"destination": destination,
		},
		metrics.NewMetrics(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Consume(ctx)

	batch := &model.Batch{BatchID: "batch-1", ExecutorID: "destination"}
	require.NoError(t, manager.Submit(ctx, batch))

	select {
	case got := <-destination.batches:
		require.Same(t, batch, got)
	case <-time.After(time.Second):
		t.Fatal("destination executor did not receive batch")
	}

	select {
	case <-source.batches:
		t.Fatal("source executor received destination batch")
	default:
	}
}

func TestManagerRejectsBatchForUnknownExecutor(t *testing.T) {
	manager := NewExecutorManager(
		zap.NewNop().Sugar(),
		map[string]Executor{
			"source": &recordingExecutor{id: "source", batches: make(chan *model.Batch, 1)},
		},
		metrics.NewMetrics(),
	)

	err := manager.Submit(context.Background(), &model.Batch{
		BatchID:    "batch-1",
		ExecutorID: "missing",
	})
	require.Error(t, err)
}
