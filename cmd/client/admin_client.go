package client

import (
	"context"
	"net/http"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
	"github.com/qujing226/kvtide/gen/go/kvtide/v1/kvtidev1connect"
)

type AdminClient struct {
	httpClient *http.Client
	endpoints  []string
	client     kvtidev1connect.AdminServiceClient
}

func NewAdminClient(endpoints []string) *AdminClient {
	transport := newLongConnTransport()
	a := &AdminClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		endpoints: endpoints,
	}
	a.dial()
	return a
}

func (a *AdminClient) dial() {
	a.client = kvtidev1connect.NewAdminServiceClient(a.httpClient, a.endpoints[0])
}

func (a *AdminClient) Health(ctx context.Context, request *v1.HealthRequest) (*v1.HealthResponse, error) {
	return a.client.Health(ctx, request)
}

func (a *AdminClient) GetRuntimeStats(ctx context.Context, request *v1.GetRuntimeStatsRequest) (*v1.GetRuntimeStatsResponse, error) {
	return a.client.GetRuntimeStats(ctx, request)
}
