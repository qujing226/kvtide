package main

import (
	"context"
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/block"
	"github.com/qujing226/kvtide/internal/executor"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
)

type runtimeExecutorStub struct {
	runtime *model.ExecutorStats
}

func (s runtimeExecutorStub) Execute(context.Context, *model.Batch) ([]*model.Event, error) {
	return nil, nil
}

func (s runtimeExecutorStub) TriggerKVPush(
	context.Context,
	*v1.TriggerKVPushRequest,
) (*v1.TriggerKVPushResponse, error) {
	return nil, nil
}

func (s runtimeExecutorStub) GetRuntimeStates() *model.ExecutorStats {
	return s.runtime
}

func TestNewBlockConfigsUsesEveryExecutorRuntime(t *testing.T) {
	configs, err := newBlockConfigs(map[string]executor.Executor{
		"executor-b": runtimeExecutorStub{runtime: &model.ExecutorStats{
			ExecutorId:   "executor-b",
			RuntimeEpoch: 12,
			BlockSize:    32,
			NumKvBlocks:  200,
		}},
		"executor-a": runtimeExecutorStub{runtime: &model.ExecutorStats{
			ExecutorId:   "executor-a",
			RuntimeEpoch: 7,
			BlockSize:    16,
			NumKvBlocks:  100,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, []block.Config{
		{
			ExecutorID:   "executor-a",
			RuntimeEpoch: 7,
			BlockSize:    16,
			NumBlocks:    100,
		},
		{
			ExecutorID:   "executor-b",
			RuntimeEpoch: 12,
			BlockSize:    32,
			NumBlocks:    200,
		},
	}, configs)
}

func TestNewBlockConfigsRejectsRuntimeIdentityMismatch(t *testing.T) {
	_, err := newBlockConfigs(map[string]executor.Executor{
		"configured-id": runtimeExecutorStub{runtime: &model.ExecutorStats{
			ExecutorId:  "runtime-id",
			BlockSize:   16,
			NumKvBlocks: 100,
		}},
	})
	require.Error(t, err)
}
