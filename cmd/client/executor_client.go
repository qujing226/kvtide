package client

import (
	"context"
	"net/http"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1/mini_llm_servev1connect"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type ExecutorClient struct {
	mini_llm_servev1connect.ExecutorServiceClient
	httpClient *http.Client
	endpoints  []string
}

func NewExecutorClient(endpoints []string, timeoutMs int) *ExecutorClient {
	transport := newLongConnTransport()
	e := &ExecutorClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeoutMs) * time.Millisecond,
		},
		endpoints: endpoints,
	}
	e.ExecutorServiceClient = mini_llm_servev1connect.NewExecutorServiceClient(e.httpClient, e.endpoints[0])
	return e
}

func (e *ExecutorClient) ExecuteBatch(ctx context.Context, request *v1.ExecuteBatchRequest) (*v1.ExecuteBatchResponse, error) {
	resp, err := e.ExecutorServiceClient.ExecuteBatch(ctx, request)
	return resp, err
}

func (e *ExecutorClient) GetRuntime() (*model.ExecutorStats, error) {
	resp, err := e.ExecutorServiceClient.GetRuntime(context.Background(), &v1.GetRuntimeRequest{})
	if err != nil {
		return nil, err
	}
	return model.RuntimeProtoToModel(resp), nil
}

func (e *ExecutorClient) ReleaseBlocks(ctx context.Context, epoch uint32, blockIds []uint32) (*v1.ReleaseBlocksResponse, error) {
	resp, err := e.ExecutorServiceClient.ReleaseBlocks(ctx, &v1.ReleaseBlocksRequest{
		RuntimeEpoch: epoch,
		BlockIds:     blockIds,
	})
	return resp, err
}
