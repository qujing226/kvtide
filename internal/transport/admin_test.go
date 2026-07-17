package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qujing226/kvtide/internal/conf"
	"github.com/qujing226/kvtide/internal/metrics"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

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
	)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()

	server.Server.Handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "http://localhost:5173", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")
}
