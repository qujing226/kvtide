package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"go.uber.org/zap"
)

const (
	CacheKeyModeNone   = ""
	CacheKeyModeUnique = "unique"
	CacheKeyModeShared = "shared"
)

type Scenario struct {
	Name           string
	Target         string
	MetricsURL     string
	Requests       int
	Concurrency    int
	Timeout        time.Duration
	Model          string
	Prompt         string
	Prompts        []string
	MaxTokens      uint32
	CacheKeyMode   string
	CacheKey       string
	WarmupRequests int
}

type BenchMetrics struct {
	BatchesTotal           float64
	QueueRejectedTotal     float64
	AvgBatchSize           float64
	AvgQueueWaitSeconds    float64
	AvgExecutionSeconds    float64
	RequestCountObserved   float64
	BatchCountObserved     float64
	ActiveRequestsFinal    float64
	InflightBatchesFinal   float64
	AvgTTFTSeconds         float64
	AvgTBTSeconds          float64
	PrefixCacheHits        float64
	PrefixCacheMisses      float64
	PrefixCacheTokensSaved float64

	batchSizeSum   float64
	batchSizeCount float64
	queueWaitSum   float64
	queueWaitCount float64
	executionSum   float64
	executionCount float64
	ttftSum        float64
	ttftCount      float64
	tbtSum         float64
	tbtCount       float64
}

type Result struct {
	Scenario      Scenario
	Success       int
	Failed        int
	TotalDuration time.Duration
	ThroughputRPS float64
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P90Latency    time.Duration
	P99Latency    time.Duration
	Metrics       BenchMetrics
}

func ScenarioPreset(name string) (Scenario, error) {
	switch name {
	case "smoke":
		return Scenario{
			Name:        name,
			Requests:    100,
			Concurrency: 10,
			Timeout:     3 * time.Second,
			Model:       "deepseek",
			Prompt:      "this is the prompt text....",
			MaxTokens:   128,
		}, nil
	case "baseline_no_batching":
		return Scenario{
			Name:        name,
			Requests:    300,
			Concurrency: 10,
			Timeout:     30 * time.Second,
			Model:       "deepseek",
			Prompt:      "this is the prompt text....",
			MaxTokens:   128,
		}, nil
	case "dynamic_default", "dynamic_fastflush":
		return Scenario{
			Name:        name,
			Requests:    1000,
			Concurrency: 100,
			Timeout:     10 * time.Second,
			Model:       "deepseek",
			Prompt:      "this is the prompt text....",
			MaxTokens:   128,
		}, nil
	case "cache_miss":
		return Scenario{
			Name:         name,
			Requests:     1000,
			Concurrency:  100,
			Timeout:      60 * time.Second,
			Model:        "deepseek",
			Prompt:       prompt(),
			MaxTokens:    128,
			CacheKeyMode: CacheKeyModeUnique,
		}, nil
	case "cache_hit":
		return Scenario{
			Name:           name,
			Requests:       1000,
			Concurrency:    100,
			Timeout:        60 * time.Second,
			Model:          "deepseek",
			Prompt:         prompt(),
			MaxTokens:      128,
			CacheKeyMode:   CacheKeyModeShared,
			CacheKey:       "shared-prefix",
			WarmupRequests: 1,
		}, nil
	case "mixed_prompt":
		return Scenario{
			Name:        name,
			Requests:    1000,
			Concurrency: 100,
			Timeout:     60 * time.Second,
			Model:       "deepseek",
			Prompts: []string{
				"short prompt",
				"medium prompt with enough content to create a different prefill cost profile for the scheduler",
				prompt(),
			},
			MaxTokens:    128,
			CacheKeyMode: CacheKeyModeUnique,
		}, nil
	default:
		return Scenario{}, fmt.Errorf("unsupported mode: %s", name)
	}
}

func RunScenario(logger *zap.Logger, scenario Scenario) (Result, error) {
	sugar := logger.Sugar()
	inferenceClient := client.NewClientWithTimeout([]string{scenario.Target}, scenario.Timeout+2*time.Second)

	if err := runWarmupRequests(inferenceClient, scenario); err != nil {
		return Result{}, err
	}

	beforeMetrics, err := fetchBenchMetrics(scenario.MetricsURL)
	if err != nil {
		return Result{}, err
	}

	var (
		wg        sync.WaitGroup
		sem       = make(chan struct{}, scenario.Concurrency)
		latMu     sync.Mutex
		latencies = make([]time.Duration, 0, scenario.Requests)
		successMu sync.Mutex
		success   int
		failed    int
	)

	runStart := time.Now()
	for i := 0; i < scenario.Requests; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			start := time.Now()
			resp, err := inferenceClient.Generate(context.Background(), buildGenerateRequest(scenario, i))
			latency := time.Since(start)

			latMu.Lock()
			latencies = append(latencies, latency)
			latMu.Unlock()

			successMu.Lock()
			defer successMu.Unlock()

			if err != nil {
				failed++
				sugar.Errorw("request failed", "index", i, "err", err)
				return
			}
			if resp == nil {
				failed++
				sugar.Errorw("request returned nil response", "index", i)
				return
			}
			if resp.ErrorMessage != "" {
				failed++
				sugar.Errorw("request returned error response",
					"index", i,
					"request_id", resp.RequestId,
					"finish_reason", resp.FinishReason.String(),
					"error_message", resp.ErrorMessage,
				)
				return
			}
			if resp.FinishReason != v1.FinishReasonStop {
				failed++
				sugar.Errorw("request returned unexpected finish reason",
					"index", i,
					"request_id", resp.RequestId,
					"finish_reason", resp.FinishReason.String(),
					"error_message", resp.ErrorMessage,
				)
				return
			}
			success++
		}(i)
	}
	wg.Wait()

	totalDuration := time.Since(runStart)
	afterMetrics, err := fetchBenchMetrics(scenario.MetricsURL)
	if err != nil {
		return Result{}, err
	}
	serverMetrics := deltaBenchMetrics(beforeMetrics, afterMetrics)

	slices.Sort(latencies)
	return Result{
		Scenario:      scenario,
		Success:       success,
		Failed:        failed,
		TotalDuration: totalDuration,
		ThroughputRPS: float64(success) / totalDuration.Seconds(),
		AvgLatency:    avgDuration(latencies),
		P50Latency:    percentileDuration(latencies, 50),
		P90Latency:    percentileDuration(latencies, 90),
		P99Latency:    percentileDuration(latencies, 99),
		Metrics:       serverMetrics,
	}, nil
}

type generateClient interface {
	Generate(ctx context.Context, req *v1.GenerateRequest) (*v1.GenerateResponse, error)
}

func runWarmupRequests(inferenceClient generateClient, scenario Scenario) error {
	for i := 0; i < scenario.WarmupRequests; i++ {
		req := buildGenerateRequest(scenario, i)
		req.RequestId = fmt.Sprintf("bench-%s-warmup-%06d", scenario.Name, i)

		resp, err := inferenceClient.Generate(context.Background(), req)
		if err != nil {
			return err
		}
		if resp == nil {
			return fmt.Errorf("warmup request returned nil response")
		}
		if resp.ErrorMessage != "" {
			return fmt.Errorf("warmup request returned error response: %s", resp.ErrorMessage)
		}
		if resp.FinishReason != v1.FinishReasonStop {
			return fmt.Errorf("warmup request returned unexpected finish reason: %s", resp.FinishReason.String())
		}
	}
	return nil
}

func durationToMilliseconds(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	return uint32(d / time.Millisecond)
}

func buildGenerateRequest(scenario Scenario, index int) *v1.GenerateRequest {
	return &v1.GenerateRequest{
		RequestId: fmt.Sprintf("bench-%s-%06d", scenario.Name, index),
		Model:     scenario.Model,
		Prompt:    scenario.promptFor(index),
		MaxTokens: scenario.MaxTokens,
		TimeoutMs: durationToMilliseconds(scenario.Timeout),
		UserId:    scenario.userId(index),
		Labels: map[string]string{
			"scenario": scenario.Name,
		},
	}
}

func (s Scenario) promptFor(index int) string {
	if len(s.Prompts) == 0 {
		return s.Prompt
	}
	return s.Prompts[index%len(s.Prompts)]
}

func (s Scenario) userId(index int) string {
	switch s.CacheKeyMode {
	case CacheKeyModeShared:
		if s.CacheKey != "" {
			return s.CacheKey
		}
		return fmt.Sprintf("bench-%s-cache-shared", s.Name)
	case CacheKeyModeUnique:
		return fmt.Sprintf("bench-%s-cache-%06d", s.Name, index)
	default:
		return ""
	}
}

func prompt() string {
	return "Currently, vLLM utilizes its own implementation of a multi-head query attention kernel. This benchmark prompt is intentionally longer so prefill work is visible in the scheduler and cache metrics. Currently, vLLM utilizes its own implementation of a multi-head query attention kernel. This benchmark prompt is intentionally longer so prefill work is visible in the scheduler and cache metrics. Currently, vLLM utilizes its own implementation of a multi-head query attention kernel. This benchmark prompt is intentionally longer so prefill work is visible in the scheduler and cache metrics."
}

func fetchBenchMetrics(metricsURL string) (BenchMetrics, error) {
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get(metricsURL)
	if err != nil {
		return BenchMetrics{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BenchMetrics{}, err
	}
	return parseBenchMetrics(string(body)), nil
}

func parseBenchMetrics(body string) BenchMetrics {
	var (
		metrics        BenchMetrics
		batchSizeSum   float64
		batchSizeCount float64
		queueWaitSum   float64
		queueWaitCount float64
		executionSum   float64
		executionCount float64
		ttftSum        float64
		ttftCount      float64
		tbtSum         float64
		tbtCount       float64
		requestCount   float64
	)

	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, value, ok := parseMetricLine(line)
		if !ok {
			continue
		}

		switch {
		case strings.HasPrefix(name, "llm_batches_total"):
			metrics.BatchesTotal += value
		case name == "llm_queue_rejected_total":
			metrics.QueueRejectedTotal = value
		case metricBaseName(name) == "llm_batch_size_sum":
			batchSizeSum = value
		case metricBaseName(name) == "llm_batch_size_count":
			batchSizeCount = value
		case strings.HasPrefix(name, "llm_execution_seconds_sum"):
			executionSum += value
		case strings.HasPrefix(name, "llm_execution_seconds_count"):
			executionCount += value
		case name == "llm_queue_wait_seconds_sum":
			queueWaitSum = value
		case name == "llm_queue_wait_seconds_count":
			queueWaitCount = value
		case strings.HasPrefix(name, "llm_requests_total"):
			requestCount += value
		case name == "llm_ttft_seconds_sum":
			ttftSum = value
		case name == "llm_ttft_seconds_count":
			ttftCount = value
		case name == "llm_tbt_seconds_sum":
			tbtSum = value
		case name == "llm_tbt_seconds_count":
			tbtCount = value
		case strings.HasPrefix(name, "llm_prefix_cache_requests_total"):
			if strings.Contains(name, `status="hit"`) {
				metrics.PrefixCacheHits += value
			}
			if strings.Contains(name, `status="miss"`) {
				metrics.PrefixCacheMisses += value
			}
		case name == "llm_prefix_cache_tokens_saved_total":
			metrics.PrefixCacheTokensSaved = value
		case name == "llm_active_requests":
			metrics.ActiveRequestsFinal = value
		case name == "llm_inflight_batches":
			metrics.InflightBatchesFinal = value
		}
	}

	metrics.batchSizeSum = batchSizeSum
	metrics.batchSizeCount = batchSizeCount
	metrics.queueWaitSum = queueWaitSum
	metrics.queueWaitCount = queueWaitCount
	metrics.executionSum = executionSum
	metrics.executionCount = executionCount
	metrics.ttftSum = ttftSum
	metrics.ttftCount = ttftCount
	metrics.tbtSum = tbtSum
	metrics.tbtCount = tbtCount
	metrics.RequestCountObserved = requestCount
	return finalizeBenchMetrics(metrics)
}

func deltaBenchMetrics(before, after BenchMetrics) BenchMetrics {
	return finalizeBenchMetrics(BenchMetrics{
		BatchesTotal:           counterDelta(before.BatchesTotal, after.BatchesTotal),
		QueueRejectedTotal:     counterDelta(before.QueueRejectedTotal, after.QueueRejectedTotal),
		RequestCountObserved:   counterDelta(before.RequestCountObserved, after.RequestCountObserved),
		ActiveRequestsFinal:    after.ActiveRequestsFinal,
		InflightBatchesFinal:   after.InflightBatchesFinal,
		PrefixCacheHits:        counterDelta(before.PrefixCacheHits, after.PrefixCacheHits),
		PrefixCacheMisses:      counterDelta(before.PrefixCacheMisses, after.PrefixCacheMisses),
		PrefixCacheTokensSaved: counterDelta(before.PrefixCacheTokensSaved, after.PrefixCacheTokensSaved),
		batchSizeSum:           counterDelta(before.batchSizeSum, after.batchSizeSum),
		batchSizeCount:         counterDelta(before.batchSizeCount, after.batchSizeCount),
		queueWaitSum:           counterDelta(before.queueWaitSum, after.queueWaitSum),
		queueWaitCount:         counterDelta(before.queueWaitCount, after.queueWaitCount),
		executionSum:           counterDelta(before.executionSum, after.executionSum),
		executionCount:         counterDelta(before.executionCount, after.executionCount),
		ttftSum:                counterDelta(before.ttftSum, after.ttftSum),
		ttftCount:              counterDelta(before.ttftCount, after.ttftCount),
		tbtSum:                 counterDelta(before.tbtSum, after.tbtSum),
		tbtCount:               counterDelta(before.tbtCount, after.tbtCount),
	})
}

func finalizeBenchMetrics(metrics BenchMetrics) BenchMetrics {
	if metrics.batchSizeCount > 0 {
		metrics.AvgBatchSize = metrics.batchSizeSum / metrics.batchSizeCount
		metrics.BatchCountObserved = metrics.batchSizeCount
	}
	if metrics.queueWaitCount > 0 {
		metrics.AvgQueueWaitSeconds = metrics.queueWaitSum / metrics.queueWaitCount
	}
	if metrics.executionCount > 0 {
		metrics.AvgExecutionSeconds = metrics.executionSum / metrics.executionCount
	}
	if metrics.ttftCount > 0 {
		metrics.AvgTTFTSeconds = metrics.ttftSum / metrics.ttftCount
	}
	if metrics.tbtCount > 0 {
		metrics.AvgTBTSeconds = metrics.tbtSum / metrics.tbtCount
	}
	return metrics
}

func counterDelta(before, after float64) float64 {
	if after < before {
		return after
	}
	return after - before
}

func metricBaseName(name string) string {
	labelStart := strings.IndexByte(name, '{')
	if labelStart < 0 {
		return name
	}
	return name[:labelStart]
}

func parseMetricLine(line string) (string, float64, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", 0, false
	}
	value, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return "", 0, false
	}
	return fields[0], value, true
}

func avgDuration(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	return total / time.Duration(len(ds))
}

func percentileDuration(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	if p <= 0 {
		return ds[0]
	}
	if p >= 100 {
		return ds[len(ds)-1]
	}
	idx := int(math.Ceil((p/100)*float64(len(ds)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(ds) {
		idx = len(ds) - 1
	}
	return ds[idx]
}

func printResult(w io.Writer, result Result) {
	fmt.Fprintf(w, "scenario=%s requests=%d success=%d failed=%d concurrency=%d total=%s throughput_rps=%.2f\n",
		result.Scenario.Name,
		result.Scenario.Requests,
		result.Success,
		result.Failed,
		result.Scenario.Concurrency,
		result.TotalDuration.Round(time.Millisecond),
		result.ThroughputRPS,
	)
	fmt.Fprintf(w, "latency avg=%s p50=%s p90=%s p99=%s\n",
		result.AvgLatency.Round(time.Millisecond),
		result.P50Latency.Round(time.Millisecond),
		result.P90Latency.Round(time.Millisecond),
		result.P99Latency.Round(time.Millisecond),
	)
	fmt.Fprintf(w, "metrics batches_total=%.0f avg_batch_size=%.2f avg_queue_wait_s=%.4f avg_execution_s=%.4f queue_rejected=%.0f active_requests=%.0f inflight_batches=%.0f observed_requests=%.0f\n",
		result.Metrics.BatchesTotal,
		result.Metrics.AvgBatchSize,
		result.Metrics.AvgQueueWaitSeconds,
		result.Metrics.AvgExecutionSeconds,
		result.Metrics.QueueRejectedTotal,
		result.Metrics.ActiveRequestsFinal,
		result.Metrics.InflightBatchesFinal,
		result.Metrics.RequestCountObserved,
	)
	fmt.Fprintf(w, "avg_ttft_s=%.4f avg_tbt_s=%.4f prefix_cache_hits=%.0f prefix_cache_misses=%.0f prefix_cache_tokens_saved=%.0f\n",
		result.Metrics.AvgTTFTSeconds,
		result.Metrics.AvgTBTSeconds,
		result.Metrics.PrefixCacheHits,
		result.Metrics.PrefixCacheMisses,
		result.Metrics.PrefixCacheTokensSaved,
	)
}
