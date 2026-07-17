package client

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/gen/go/kvtide/v1/kvtidev1connect"
)

type InferenceClient struct {
	httpClient      *http.Client
	endpoints       []string
	inferenceClient kvtidev1connect.InferenceServiceClient
}

func NewClient(endpoints []string, timeout time.Duration) *InferenceClient {
	transport := newLongConnTransport()

	c := &InferenceClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		endpoints: endpoints,
	}
	c.dial()
	return c
}

func (c *InferenceClient) Generate(ctx context.Context, request *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	resp, err := c.inferenceClient.Generate(ctx, request)
	return resp, err
}

func (c *InferenceClient) GenerateStream(
	ctx context.Context,
	req *v1.GenerateRequest,
) (*connect.ServerStreamForClient[v1.GenerateResponseChunk], error) {
	stream, err := c.inferenceClient.GenerateStream(ctx, req)
	return stream, err
}

func (c *InferenceClient) dial() {
	c.inferenceClient = kvtidev1connect.NewInferenceServiceClient(c.httpClient, c.endpoints[0])
}
