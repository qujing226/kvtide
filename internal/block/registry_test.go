package block

import (
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRegistryBuildsIndependentManagersInStableOrder(t *testing.T) {
	blockRegistry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		[]Config{
			{ExecutorID: "executor-b", BlockSize: 16, NumBlocks: 8},
			{ExecutorID: "executor-a", BlockSize: 16, NumBlocks: 4},
		},
	)
	require.NoError(t, err)

	r := blockRegistry.(*registry)
	executorA := r.managers["executor-a"]
	executorB := r.managers["executor-b"]
	require.NotSame(t, executorA, executorB)
	require.Len(t, executorA.blocks, 4)
	require.Len(t, executorB.blocks, 8)

	ids := blockRegistry.ExecutorIDs()
	require.Equal(t, []string{"executor-a", "executor-b"}, ids)
	ids[0] = "changed-by-caller"
	require.Equal(
		t,
		[]string{"executor-a", "executor-b"},
		blockRegistry.ExecutorIDs(),
	)
}

func TestRegistryRejectsInvalidExecutorConfigs(t *testing.T) {
	tests := []struct {
		name    string
		configs []Config
	}{
		{name: "empty registry"},
		{
			name: "empty executor ID",
			configs: []Config{
				{BlockSize: 16, NumBlocks: 4},
			},
		},
		{
			name: "duplicate executor ID",
			configs: []Config{
				{ExecutorID: "executor-a", BlockSize: 16, NumBlocks: 4},
				{ExecutorID: "executor-a", BlockSize: 16, NumBlocks: 8},
			},
		},
		{
			name: "invalid manager config",
			configs: []Config{
				{ExecutorID: "executor-a", BlockSize: 0, NumBlocks: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRegistry(
				zap.NewNop().Sugar(),
				metrics.NewMetrics(),
				tt.configs,
			)
			require.Error(t, err)
		})
	}
}

func TestRegistryRejectsUnknownExecutor(t *testing.T) {
	registry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		[]Config{
			{ExecutorID: "executor-a", BlockSize: 16, NumBlocks: 4},
		},
	)
	require.NoError(t, err)

	err = registry.Commit("missing", "work-1")
	require.Error(t, err)
}

func TestRegistryRoutesBlockLifecycleByExecutorID(t *testing.T) {
	registry, err := NewRegistry(
		zap.NewNop().Sugar(),
		metrics.NewMetrics(),
		[]Config{
			{ExecutorID: "executor-a", BlockSize: 2, NumBlocks: 4},
			{ExecutorID: "executor-b", BlockSize: 2, NumBlocks: 4},
		},
	)
	require.NoError(t, err)

	req := &model.Request{
		RequestId:  "request-1",
		ExecutorID: "executor-b",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3},
	}
	match, err := registry.MatchPrefix(req)
	require.NoError(t, err)
	require.False(t, match.Hit)

	work := &model.WorkItem{
		WorkId:       "work-1",
		RequestId:    req.RequestId,
		ExecutorID:   req.ExecutorID,
		Phase:        v1.WorkPhasePrefill,
		Cache:        match,
		TokenIDs:     req.TokenIDs,
		NumNewTokens: 3,
	}
	allocated, err := registry.AllocateBlocks(work)
	require.NoError(t, err)
	require.True(t, allocated)
	require.NoError(t, registry.Commit("executor-b", work.WorkId))
	require.NoError(t, registry.FreeRequest("executor-b", req.RequestId))

	followup := &model.Request{
		RequestId:  "request-2",
		ExecutorID: "executor-b",
		CacheSalt:  "shared-prefix",
		TokenIDs:   []uint32{1, 2, 3},
	}
	match, err = registry.MatchPrefix(followup)
	require.NoError(t, err)
	require.True(t, match.Hit)
}
