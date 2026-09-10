package executor

import (
	"context"
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
)

type transferExecutorStub struct {
	request *v1.TriggerKVPushRequest
}

func (s *transferExecutorStub) Execute(context.Context, *model.Batch) ([]*model.Event, error) {
	return nil, nil
}

func (s *transferExecutorStub) GetRuntimeStates() *model.ExecutorStats {
	return &model.ExecutorStats{ExecutorId: "source"}
}

func (s *transferExecutorStub) TriggerKVPush(
	_ context.Context,
	request *v1.TriggerKVPushRequest,
) (*v1.TriggerKVPushResponse, error) {
	s.request = request
	return &v1.TriggerKVPushResponse{TransferId: request.TransferId}, nil
}

func TestManagerForwardsKVPushCommandToSourceExecutor(t *testing.T) {
	source := &transferExecutorStub{}
	manager := &executorManager{executors: map[string]Executor{"source": source}}
	request := &v1.TriggerKVPushRequest{
		TransferId:       "transfer-1",
		SourceExecutorId: "source",
		BlockHashes:      []string{"hash-from-engine"},
	}

	response, err := manager.TriggerKVPush(context.Background(), request)

	require.NoError(t, err)
	require.Same(t, request, source.request)
	require.Equal(t, "transfer-1", response.TransferId)
}
