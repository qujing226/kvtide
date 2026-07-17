package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type runtimeStateProviderStub struct {
	states map[string]*model.ExecutorStats
}

func (s runtimeStateProviderStub) GetRuntimeStates() map[string]*model.ExecutorStats {
	return s.states
}

func TestAdminMetricsAllowsConfiguredOrigin(t *testing.T) {
	server := NewAdminService(
		zap.NewNop().Sugar(),
		&conf.Conf{
			Server: conf.ServerConf{
				AdminPort:      8801,
				AllowedOrigins: []string{"http://localhost:5173"},
			},
		},
		metrics.NewMetrics(),
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()

	server.Server.Handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "http://localhost:5173", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")
}

func TestAdminGetExecutorsReturnsExecutorSnapshots(t *testing.T) {
	service := &adminService{
		executors: runtimeStateProviderStub{states: map[string]*model.ExecutorStats{
			"executor-qwen": {
				ExecutorId:           "executor-qwen",
				RuntimeEpoch:         42,
				ModelId:              "Qwen/Qwen3-0.6B",
				ModelType:            "qwen3",
				Dtype:                "float32",
				DeviceType:           "cpu",
				TensorParallelSize:   1,
				BlockSize:            16,
				NumKvBlocks:          146,
				NumHiddenLayers:      28,
				NumKvHeads:           8,
				HeadDim:              128,
				TotalMemoryBytes:     8_000_000_000,
				AvailableMemoryBytes: 2_000_000_000,
				KVCacheBytes:         512_000_000,
			},
		}},
	}

	response, err := service.GetExecutors(context.Background(), &v1.GetExecutorsRequest{})

	require.NoError(t, err)
	require.Len(t, response.Executors, 1)
	runtime := response.Executors[0]
	require.Equal(t, "executor-qwen", runtime.ExecutorId)
	require.Equal(t, uint32(42), runtime.RuntimeEpoch)
	require.Equal(t, "Qwen/Qwen3-0.6B", runtime.ModelId)
	require.Equal(t, uint64(512_000_000), runtime.KvCacheBytes)
}
