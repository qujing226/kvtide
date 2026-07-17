package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCORSAllowsConfiguredOriginPreflight(t *testing.T) {
	handler := withCORS(
		[]string{"http://localhost:5173"},
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("preflight should not reach the application handler")
		}),
	)
	req := httptest.NewRequest(http.MethodOptions, "/kvtide.v1.InferenceService/GenerateStream", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type,connect-protocol-version")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "http://localhost:5173", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
	require.Equal(t, "content-type,connect-protocol-version", recorder.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	handler := withCORS(
		[]string{"http://localhost:5173"},
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	req := httptest.NewRequest(http.MethodOptions, "/kvtide.v1.InferenceService/GenerateStream", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSAddsHeadersToStreamingResponse(t *testing.T) {
	handler := withCORS(
		[]string{"http://127.0.0.1:5173"},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	req := httptest.NewRequest(http.MethodPost, "/kvtide.v1.InferenceService/GenerateStream", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "http://127.0.0.1:5173", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, recorder.Header().Values("Vary"), "Origin")
}
