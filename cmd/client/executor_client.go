package client

import (
	"context"
	"net/http"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1/mini_llm_servev1connect"
)

type ExecutorClient struct {
	httpClient *http.Client
	endpoints  []string
	executor   mini_llm_servev1connect.ExecuteServiceClient
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
	e.dial()
	return e
}

func (e *ExecutorClient) ExecuteBatch(ctx context.Context, request *v1.ExecuteBatchRequest) (*v1.ExecuteBatchResponse, error) {
	resp, err := e.executor.ExecuteBatch(ctx, request)
	return resp, err
}

func (e *ExecutorClient) dial() {
	e.executor = mini_llm_servev1connect.NewExecuteServiceClient(e.httpClient, e.endpoints[0])
}
