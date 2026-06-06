package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPercentileDuration(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	require.Equal(t, 30*time.Millisecond, percentileDuration(latencies, 50))
	require.Equal(t, 50*time.Millisecond, percentileDuration(latencies, 90))
	require.Equal(t, 50*time.Millisecond, percentileDuration(latencies, 99))
}

func TestParseBenchMetrics(t *testing.T) {
	raw := `
# HELP llm_batch_size Number of requests in a batch
llm_batch_size_sum{phase="mixed"} 101
llm_batch_size_count{phase="mixed"} 11
llm_batches_total{executor="mock-python"} 11
llm_execution_seconds_sum{executor="mock-python"} 13.938
llm_execution_seconds_count{executor="mock-python"} 101
llm_queue_wait_seconds_sum 9.996
llm_queue_wait_seconds_count 101
llm_queue_rejected_total 0
llm_active_requests 0
llm_inflight_batches 0
llm_requests_total{executor="mock-python",status="ok"} 101
llm_ttft_seconds_sum 2.5
llm_ttft_seconds_count 10
llm_tbt_seconds_sum 5
llm_tbt_seconds_count 20
llm_prefix_cache_requests_total{status="hit"} 7
llm_prefix_cache_requests_total{status="miss"} 3
llm_prefix_cache_tokens_saved_total 56
`

	metrics := parseBenchMetrics(raw)
	require.Equal(t, 11.0, metrics.BatchesTotal)
	require.InDelta(t, 9.1818, metrics.AvgBatchSize, 0.001)
	require.InDelta(t, 0.09897, metrics.AvgQueueWaitSeconds, 0.001)
	require.InDelta(t, 0.138, metrics.AvgExecutionSeconds, 0.001)
	require.Equal(t, 101.0, metrics.RequestCountObserved)
	require.Equal(t, 0.0, metrics.ActiveRequestsFinal)
	require.Equal(t, 0.0, metrics.InflightBatchesFinal)
	require.InDelta(t, 0.25, metrics.AvgTTFTSeconds, 0.001)
	require.InDelta(t, 0.25, metrics.AvgTBTSeconds, 0.001)
	require.Equal(t, 7.0, metrics.PrefixCacheHits)
	require.Equal(t, 3.0, metrics.PrefixCacheMisses)
	require.Equal(t, 56.0, metrics.PrefixCacheTokensSaved)
}

func TestDeltaBenchMetricsUsesCountersAndHistogramDeltas(t *testing.T) {
	before := parseBenchMetrics(`
llm_batch_size_sum{phase="mixed"} 100
llm_batch_size_count{phase="mixed"} 10
llm_batches_total{executor="mock-python",phase="mixed"} 10
llm_execution_seconds_sum{executor="mock-python"} 1
llm_execution_seconds_count{executor="mock-python"} 10
llm_queue_wait_seconds_sum 0.5
llm_queue_wait_seconds_count 10
llm_requests_total{executor="mock-python",status="ok"} 10
llm_ttft_seconds_sum 2
llm_ttft_seconds_count 10
llm_tbt_seconds_sum 3
llm_tbt_seconds_count 10
llm_prefix_cache_requests_total{status="hit"} 5
llm_prefix_cache_requests_total{status="miss"} 5
llm_prefix_cache_tokens_saved_total 100
llm_queue_rejected_total 1
llm_active_requests 7
llm_inflight_batches 3
`)
	after := parseBenchMetrics(`
llm_batch_size_sum{phase="mixed"} 180
llm_batch_size_count{phase="mixed"} 20
llm_batches_total{executor="mock-python",phase="mixed"} 20
llm_execution_seconds_sum{executor="mock-python"} 3
llm_execution_seconds_count{executor="mock-python"} 20
llm_queue_wait_seconds_sum 1.5
llm_queue_wait_seconds_count 20
llm_requests_total{executor="mock-python",status="ok"} 20
llm_ttft_seconds_sum 5
llm_ttft_seconds_count 20
llm_tbt_seconds_sum 7
llm_tbt_seconds_count 20
llm_prefix_cache_requests_total{status="hit"} 14
llm_prefix_cache_requests_total{status="miss"} 6
llm_prefix_cache_tokens_saved_total 280
llm_queue_rejected_total 2
llm_active_requests 0
llm_inflight_batches 0
`)

	metrics := deltaBenchMetrics(before, after)

	require.Equal(t, 10.0, metrics.BatchesTotal)
	require.InDelta(t, 8.0, metrics.AvgBatchSize, 0.001)
	require.InDelta(t, 0.2, metrics.AvgExecutionSeconds, 0.001)
	require.InDelta(t, 0.1, metrics.AvgQueueWaitSeconds, 0.001)
	require.Equal(t, 10.0, metrics.RequestCountObserved)
	require.InDelta(t, 0.3, metrics.AvgTTFTSeconds, 0.001)
	require.InDelta(t, 0.4, metrics.AvgTBTSeconds, 0.001)
	require.Equal(t, 9.0, metrics.PrefixCacheHits)
	require.Equal(t, 1.0, metrics.PrefixCacheMisses)
	require.Equal(t, 180.0, metrics.PrefixCacheTokensSaved)
	require.Equal(t, 1.0, metrics.QueueRejectedTotal)
	require.Equal(t, 0.0, metrics.ActiveRequestsFinal)
	require.Equal(t, 0.0, metrics.InflightBatchesFinal)
}

func TestScenarioPreset(t *testing.T) {
	baseline, err := ScenarioPreset("baseline_no_batching")
	require.NoError(t, err)
	require.Equal(t, 300, baseline.Requests)
	require.Equal(t, 10, baseline.Concurrency)
	require.Equal(t, 30*time.Second, baseline.Timeout)

	dynamicDefault, err := ScenarioPreset("dynamic_default")
	require.NoError(t, err)
	require.Equal(t, 1000, dynamicDefault.Requests)
	require.Equal(t, 100, dynamicDefault.Concurrency)
	require.Equal(t, 10*time.Second, dynamicDefault.Timeout)

	smoke, err := ScenarioPreset("smoke")
	require.NoError(t, err)
	require.Equal(t, 100, smoke.Requests)
	require.Equal(t, 10, smoke.Concurrency)
	require.Equal(t, 3*time.Second, smoke.Timeout)

	cacheMiss, err := ScenarioPreset("cache_miss")
	require.NoError(t, err)
	require.Equal(t, CacheKeyModeUnique, cacheMiss.CacheKeyMode)
	require.Equal(t, 1000, cacheMiss.Requests)

	cacheHit, err := ScenarioPreset("cache_hit")
	require.NoError(t, err)
	require.Equal(t, CacheKeyModeShared, cacheHit.CacheKeyMode)
	require.Equal(t, 1, cacheHit.WarmupRequests)
	require.Equal(t, 100, cacheHit.Concurrency)

	mixedPrompt, err := ScenarioPreset("mixed_prompt")
	require.NoError(t, err)
	require.Len(t, mixedPrompt.Prompts, 3)
	require.Equal(t, CacheKeyModeUnique, mixedPrompt.CacheKeyMode)
}

func TestBuildGenerateRequestUsesStage2ScenarioFields(t *testing.T) {
	scenario := Scenario{
		Name:         "cache_miss",
		Model:        "deepseek",
		Prompt:       "fallback prompt",
		Prompts:      []string{"short prompt", "a much longer prompt used by the mixed prompt benchmark"},
		MaxTokens:    128,
		Timeout:      10 * time.Second,
		CacheKeyMode: CacheKeyModeUnique,
	}

	req := buildGenerateRequest(scenario, 1)

	require.Equal(t, "bench-cache_miss-000001", req.RequestId)
	require.Equal(t, "deepseek", req.Model)
	require.Equal(t, "a much longer prompt used by the mixed prompt benchmark", req.Prompt)
	require.Equal(t, uint32(128), req.MaxTokens)
	require.Equal(t, uint32(10000), req.TimeoutMs)
	require.Equal(t, "bench-cache_miss-cache-000001", req.UserId)
	require.Equal(t, "cache_miss", req.Labels["scenario"])
}

func TestBuildGenerateRequestUsesSharedCacheKey(t *testing.T) {
	scenario := Scenario{
		Name:         "cache_hit",
		Model:        "deepseek",
		Prompt:       "shared prompt",
		MaxTokens:    128,
		Timeout:      10 * time.Second,
		CacheKeyMode: CacheKeyModeShared,
		CacheKey:     "shared-prefix",
	}

	req0 := buildGenerateRequest(scenario, 0)
	req1 := buildGenerateRequest(scenario, 1)

	require.Equal(t, "shared-prefix", req0.UserId)
	require.Equal(t, "shared-prefix", req1.UserId)
}
