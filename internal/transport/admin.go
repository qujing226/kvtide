package transport

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/gen/go/kvtide/v1/kvtidev1connect"
	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/executor"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/qujing226/kvtide/internal/model"
	"go.uber.org/zap"
	brotli "go.withmatt.com/connect-brotli"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type adminService struct {
	l         *zap.SugaredLogger
	metrics   metrics.Metrics
	executors runtimeStateProvider
}

type runtimeStateProvider interface {
	GetRuntimeStates() map[string]*model.ExecutorStats
}

type AdminServer struct {
	Server *http.Server
}

func NewAdminService(l *zap.SugaredLogger, cfg *conf.Conf, metrics metrics.Metrics, executors executor.Manager) *AdminServer {
	a := &adminService{
		l:         l,
		metrics:   metrics,
		executors: executors,
	}

	mux := http.NewServeMux()
	path, handler := kvtidev1connect.NewAdminServiceHandler(
		a,
		connect.WithInterceptors(),
		connect.WithCompressMinBytes(CompressionMinBytes),
		brotli.WithCompression(),
	)
	mux.Handle(path, handler)
	mux.Handle("/metrics", metrics.Handler())

	h2cHandler := h2c.NewHandler(mux, &http2.Server{})
	handlerWithCORS := withCORS(cfg.Server.AllowedOrigins, h2cHandler)

	port := cfg.Server.AdminPort
	if port < 1024 || port > 65535 {
		l.Errorf("port %d is out of range [1024, 65535]", port)
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handlerWithCORS,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       15 * time.Second,
	}

	return &AdminServer{Server: srv}
}

func (a *adminService) Health(ctx context.Context, request *v1.HealthRequest) (*v1.HealthResponse, error) {
	return &v1.HealthResponse{
		Status: "ok",
	}, nil
}

func (a *adminService) GetRuntimeStats(ctx context.Context, request *v1.GetRuntimeStatsRequest) (*v1.GetRuntimeStatsResponse, error) {
	snapshot := a.metrics.Snapshot()
	return &v1.GetRuntimeStatsResponse{
		PrefillQueueLen:  uint32(snapshot.PrefillQueueLength),
		DecodeQueueLen:   uint32(snapshot.DecodeQueueLength),
		InflightRequests: uint32(snapshot.ActiveRequests),
		InflightBatches:  uint32(snapshot.InflightBatches),
		BusyExecutors:    uint32(snapshot.BusyExecutors),
		IdleExecutors:    uint32(snapshot.IdleExecutors),
	}, nil
}

func (a *adminService) GetExecutors(ctx context.Context, request *v1.GetExecutorsRequest) (*v1.GetExecutorsResponse, error) {
	states := a.executors.GetRuntimeStates()
	executorIDs := make([]string, 0, len(states))
	for executorID := range states {
		executorIDs = append(executorIDs, executorID)
	}
	sort.Strings(executorIDs)

	executors := make([]*v1.GetRuntimeResponse, 0, len(executorIDs))
	for _, executorID := range executorIDs {
		state := states[executorID]
		executors = append(executors, &v1.GetRuntimeResponse{
			ExecutorId:           state.ExecutorId,
			RuntimeEpoch:         state.RuntimeEpoch,
			ModelId:              state.ModelId,
			ModelType:            state.ModelType,
			Dtype:                state.Dtype,
			DeviceType:           state.DeviceType,
			TensorParallelSize:   state.TensorParallelSize,
			BlockSize:            state.BlockSize,
			NumKvBlocks:          state.NumKvBlocks,
			NumHiddenLayers:      state.NumHiddenLayers,
			NumKvHeads:           state.NumKvHeads,
			HeadDim:              state.HeadDim,
			TotalMemoryBytes:     state.TotalMemoryBytes,
			AvailableMemoryBytes: state.AvailableMemoryBytes,
			KvCacheBytes:         state.KVCacheBytes,
		})
	}

	return &v1.GetExecutorsResponse{Executors: executors}, nil
}
