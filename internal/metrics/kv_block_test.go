package metrics

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockMetricsAreScopedByExecutor(t *testing.T) {
	m := NewMetrics()
	m.ObserveBlockStats("executor-a", 1, 9, 2)
	m.ObserveBlockStats("executor-b", 3, 17, 4)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	response, err := io.ReadAll(recorder.Result().Body)
	require.NoError(t, err)
	metricsText := string(response)

	require.True(t, strings.Contains(metricsText,
		`llm_kv_blocks{executor="executor-a",state="active"} 1`))
	require.True(t, strings.Contains(metricsText,
		`llm_kv_blocks{executor="executor-b",state="active"} 3`))
	require.True(t, strings.Contains(metricsText,
		`llm_kv_blocks{executor="executor-a",state="free"} 9`))
	require.True(t, strings.Contains(metricsText,
		`llm_kv_blocks{executor="executor-b",state="free"} 17`))
}
