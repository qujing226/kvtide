package main

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "github.com/qujing226/kvtide/gen/go/kvtide/v1"
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
llm_batch_items_total{phase="prefill"} 10
llm_batch_items_total{phase="decode"} 80
llm_kv_blocks{state="active"} 2
llm_kv_blocks{state="free"} 1022
llm_kv_blocks{state="cached"} 8
llm_kv_allocation_failures_total 4
llm_prefix_cache_evictions_total 5
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
	require.Equal(t, 10.0, metrics.PrefillItemsTotal)
	require.Equal(t, 80.0, metrics.DecodeItemsTotal)
	require.Equal(t, 2.0, metrics.KVBlocksActiveFinal)
	require.Equal(t, 1022.0, metrics.KVBlocksFreeFinal)
	require.Equal(t, 8.0, metrics.KVBlocksCachedFinal)
	require.Equal(t, 4.0, metrics.KVAllocationFailures)
	require.Equal(t, 5.0, metrics.PrefixCacheEvictions)
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
llm_batch_items_total{phase="prefill"} 10
llm_batch_items_total{phase="decode"} 80
llm_kv_blocks{state="active"} 12
llm_kv_blocks{state="free"} 1012
llm_kv_blocks{state="cached"} 8
llm_kv_allocation_failures_total 1
llm_prefix_cache_evictions_total 2
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
llm_batch_items_total{phase="prefill"} 20
llm_batch_items_total{phase="decode"} 160
llm_kv_blocks{state="active"} 0
llm_kv_blocks{state="free"} 1024
llm_kv_blocks{state="cached"} 12
llm_kv_allocation_failures_total 3
llm_prefix_cache_evictions_total 7
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
	require.Equal(t, 10.0, metrics.PrefillItemsTotal)
	require.Equal(t, 80.0, metrics.DecodeItemsTotal)
	require.Equal(t, 0.0, metrics.KVBlocksActiveFinal)
	require.Equal(t, 1024.0, metrics.KVBlocksFreeFinal)
	require.Equal(t, 12.0, metrics.KVBlocksCachedFinal)
	require.Equal(t, 2.0, metrics.KVAllocationFailures)
	require.Equal(t, 5.0, metrics.PrefixCacheEvictions)
}

func TestProfilePreset(t *testing.T) {
	quick, err := ProfilePreset("quick")
	require.NoError(t, err)
	require.Equal(t, 100, quick.Requests)
	require.Equal(t, 20, quick.Concurrency)
	require.Equal(t, uint32(8), quick.MaxTokens)

	report, err := ProfilePreset("report")
	require.NoError(t, err)
	require.Equal(t, 1000, report.Requests)
	require.Equal(t, 100, report.Concurrency)
	require.Equal(t, uint32(128), report.MaxTokens)

	_, err = ProfilePreset("unknown")
	require.Error(t, err)
}

func TestScenariosForProfile(t *testing.T) {
	profile, err := ProfilePreset("quick")
	require.NoError(t, err)

	scenarios := ScenariosForProfile(profile)

	require.Equal(t, []string{"cache_miss", "cache_hit", "mixed_prompt", "block_pressure"}, scenarioNames(scenarios))
	for _, scenario := range scenarios {
		require.Equal(t, profile.Requests, scenario.Requests)
		require.Equal(t, profile.Concurrency, scenario.Concurrency)
		require.Equal(t, profile.MaxTokens, scenario.MaxTokens)
	}
	require.Equal(t, CacheKeyModeUnique, scenarios[0].CacheKeyMode)
	require.Equal(t, CacheKeyModeShared, scenarios[1].CacheKeyMode)
	require.Equal(t, 10, scenarios[1].CacheUsers)
	require.Len(t, scenarios[2].Prompts, 3)
	require.Equal(t, CacheKeyModeUnique, scenarios[3].CacheKeyMode)
}

func TestReportBlockPressureCapsConcurrencyBelowKVDeadlockBoundary(t *testing.T) {
	profile, err := ProfilePreset("report")
	require.NoError(t, err)

	scenarios := ScenariosForProfile(profile)

	require.Equal(t, 100, scenarios[0].Concurrency)
	require.Equal(t, 100, scenarios[1].Concurrency)
	require.Equal(t, 100, scenarios[2].Concurrency)
	require.Equal(t, 32, scenarios[3].Concurrency)
	require.Equal(t, 320, scenarios[3].Requests)
}

func TestBuildGenerateRequestUsesScenarioFields(t *testing.T) {
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

	require.Equal(t, "bench-cache_miss-measured-000001", req.RequestId)
	require.Equal(t, "deepseek", req.ModelId)
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
		CacheUsers:   2,
	}

	req0 := buildGenerateRequest(scenario, 0)
	req1 := buildGenerateRequest(scenario, 1)
	req2 := buildGenerateRequest(scenario, 2)

	require.Equal(t, "shared-prefix-000", req0.UserId)
	require.Equal(t, "shared-prefix-001", req1.UserId)
	require.Equal(t, "shared-prefix-000", req2.UserId)
}

func TestCacheHitWarmupPopulatesEveryCacheUser(t *testing.T) {
	client := &recordingGenerateClient{}
	scenario := Scenario{
		Name:         "cache_hit",
		Model:        "deepseek",
		Prompt:       "shared prompt",
		MaxTokens:    8,
		Timeout:      time.Second,
		CacheKeyMode: CacheKeyModeShared,
		CacheKey:     "shared-prefix",
		CacheUsers:   3,
	}

	require.NoError(t, runWarmupRequests(client, scenario))
	require.Len(t, client.requests, 3)
	require.Equal(t, "bench-cache_hit-warmup-000000", client.requests[0].RequestId)
	require.Equal(t, "bench-cache_hit-warmup-000001", client.requests[1].RequestId)
	require.Equal(t, "bench-cache_hit-warmup-000002", client.requests[2].RequestId)
	require.Equal(t, "shared-prefix-000", client.requests[0].UserId)
	require.Equal(t, "shared-prefix-001", client.requests[1].UserId)
	require.Equal(t, "shared-prefix-002", client.requests[2].UserId)
}

func TestValidateQuickResultAcceptsValidCacheHit(t *testing.T) {
	result := Result{
		Scenario: Scenario{
			Name:      "cache_hit",
			Requests:  100,
			MaxTokens: 8,
		},
		Success: 100,
		Metrics: BenchMetrics{
			RequestCountObserved: 100,
			PrefillItemsTotal:    100,
			DecodeItemsTotal:     800,
			PrefixCacheHits:      100,
			KVBlocksFreeFinal:    1024,
		},
	}

	require.NoError(t, ValidateQuickResult(result))
}

func TestValidateQuickResultReportsDeterministicRegressions(t *testing.T) {
	result := Result{
		Scenario: Scenario{
			Name:      "cache_miss",
			Requests:  100,
			MaxTokens: 8,
		},
		Success: 98,
		Failed:  2,
		Metrics: BenchMetrics{
			RequestCountObserved: 98,
			DecodeItemsTotal:     700,
			PrefixCacheHits:      1,
			PrefixCacheMisses:    97,
			QueueRejectedTotal:   2,
			ActiveRequestsFinal:  1,
			InflightBatchesFinal: 1,
			KVBlocksActiveFinal:  2,
			KVAllocationFailures: 3,
		},
	}

	err := ValidateQuickResult(result)
	require.Error(t, err)
	for _, message := range []string{
		"successful requests",
		"failed requests",
		"observed requests",
		"decode work items",
		"cache hits",
		"cache misses",
		"queue rejections",
		"active requests",
		"inflight batches",
		"active KV blocks",
		"allocation failures",
	} {
		require.True(t, strings.Contains(err.Error(), message), err.Error())
	}
}

func TestValidateQuickResultAllowsAllocationPressureInPressureScenario(t *testing.T) {
	result := Result{
		Scenario: Scenario{
			Name:      "block_pressure",
			Requests:  100,
			MaxTokens: 8,
		},
		Success: 100,
		Metrics: BenchMetrics{
			RequestCountObserved: 100,
			PrefillItemsTotal:    100,
			DecodeItemsTotal:     800,
			PrefixCacheMisses:    100,
			KVAllocationFailures: 12,
			PrefixCacheEvictions: 4,
			KVBlocksFreeFinal:    1024,
			KVBlocksCachedFinal:  20,
			KVBlocksActiveFinal:  0,
			ActiveRequestsFinal:  0,
			InflightBatchesFinal: 0,
			QueueRejectedTotal:   0,
		},
	}

	require.NoError(t, ValidateQuickResult(result))
}

func TestIsSuccessfulFinishReason(t *testing.T) {
	require.True(t, isSuccessfulFinishReason(v1.FinishReasonStop))
	require.True(t, isSuccessfulFinishReason(v1.FinishReasonLength))
	require.False(t, isSuccessfulFinishReason(v1.FinishReasonUnspecified))
	require.False(t, isSuccessfulFinishReason(v1.FinishReasonTimeout))
	require.False(t, isSuccessfulFinishReason(v1.FinishReasonError))
}

func scenarioNames(scenarios []Scenario) []string {
	names := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		names = append(names, scenario.Name)
	}
	return names
}

type recordingGenerateClient struct {
	requests []*v1.GenerateRequest
}

func (c *recordingGenerateClient) Generate(_ context.Context, req *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	c.requests = append(c.requests, req)
	return &v1.GenerateResponse{FinishReason: v1.FinishReasonStop}, nil
}
