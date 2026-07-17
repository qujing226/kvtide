import { describe, expect, it } from "vitest";

import {
  calculateMetricsWindow,
  metricsRefreshIntervalMs,
  parseMetricsSnapshot,
  resolveMetricsUrl,
} from "./metrics";

const beforeText = `
# HELP llm_active_requests Number of active requests
# TYPE llm_active_requests gauge
llm_active_requests 1
llm_prefill_queue_length 2
llm_decode_queue_length 3
llm_inflight_batches 1
llm_kv_blocks{state="active"} 8
llm_kv_blocks{state="free"} 1000
llm_kv_blocks{state="cached"} 16
llm_prefix_cache_requests_total{status="hit"} 4
llm_prefix_cache_requests_total{status="miss"} 6
llm_prefix_cache_tokens_saved_total 64
llm_batches_total{executor="mock-python",phase="mixed"} 10
llm_batch_size_sum{phase="mixed"} 18
llm_batch_size_count{phase="mixed"} 10
llm_batch_items_total{phase="prefill"} 5
llm_batch_items_total{phase="decode"} 13
llm_queue_wait_seconds_sum 0.20
llm_queue_wait_seconds_count 10
llm_execution_seconds_sum{executor="mock-python"} 1.5
llm_execution_seconds_count{executor="mock-python"} 10
llm_tbt_seconds_sum 0.8
llm_tbt_seconds_count 8
llm_queue_rejected_total 0
llm_executor_errors_total{executor="mock-python"} 1
llm_kv_allocation_failures_total 0
llm_prefix_cache_evictions_total 2
`;

const afterText = `
llm_active_requests 0
llm_prefill_queue_length 0
llm_decode_queue_length 1
llm_inflight_batches 0
llm_kv_blocks{state="active"} 4
llm_kv_blocks{state="free"} 1004
llm_kv_blocks{state="cached"} 24
llm_prefix_cache_requests_total{status="hit"} 5
llm_prefix_cache_requests_total{status="miss"} 6
llm_prefix_cache_tokens_saved_total 96
llm_batches_total{executor="mock-python",phase="mixed"} 14
llm_batch_size_sum{phase="mixed"} 26
llm_batch_size_count{phase="mixed"} 14
llm_batch_items_total{phase="prefill"} 6
llm_batch_items_total{phase="decode"} 20
llm_queue_wait_seconds_sum 0.24
llm_queue_wait_seconds_count 14
llm_execution_seconds_sum{executor="mock-python"} 1.94
llm_execution_seconds_count{executor="mock-python"} 14
llm_tbt_seconds_sum 1.1
llm_tbt_seconds_count 11
llm_queue_rejected_total 1
llm_executor_errors_total{executor="mock-python"} 1
llm_kv_allocation_failures_total 2
llm_prefix_cache_evictions_total 3
`;

describe("Prometheus metrics", () => {
  it("parses gauges and aggregates labeled counters", () => {
    const snapshot = parseMetricsSnapshot(beforeText);

    expect(snapshot.runtime).toEqual({
      prefillQueue: 2,
      decodeQueue: 3,
      activeRequests: 1,
      inflightBatches: 1,
      kvActive: 8,
      kvFree: 1000,
      kvCached: 16,
    });
    expect(snapshot.counters.prefixHits).toBe(4);
    expect(snapshot.counters.prefixMisses).toBe(6);
    expect(snapshot.counters.executorErrors).toBe(1);
  });

  it("calculates request-window deltas and histogram averages", () => {
    const window = calculateMetricsWindow(
      parseMetricsSnapshot(beforeText),
      parseMetricsSnapshot(afterText),
    );

    expect(window.prefixCache).toBe("hit");
    expect(window.tokensSaved).toBe(32);
    expect(window.batches).toBe(4);
    expect(window.averageBatchSize).toBe(2);
    expect(window.prefillItems).toBe(1);
    expect(window.decodeItems).toBe(7);
    expect(window.averageQueueWaitMs).toBeCloseTo(10);
    expect(window.averageExecutionMs).toBeCloseTo(110);
    expect(window.averageTbtMs).toBeCloseTo(100);
    expect(window.errors).toEqual({
      queueRejected: 1,
      executorErrors: 0,
      allocationFailures: 2,
      cacheEvictions: 1,
    });
  });

  it("does not produce negative deltas after a counter reset", () => {
    const before = parseMetricsSnapshot(beforeText);
    const after = parseMetricsSnapshot(
      afterText
        .replace("llm_prefix_cache_tokens_saved_total 96", "llm_prefix_cache_tokens_saved_total 2")
        .replace('llm_batches_total{executor="mock-python",phase="mixed"} 14', 'llm_batches_total{executor="mock-python",phase="mixed"} 1'),
    );

    const window = calculateMetricsWindow(before, after);

    expect(window.tokensSaved).toBe(0);
    expect(window.batches).toBe(0);
  });

  it("uses the same-origin metrics endpoint", () => {
    expect(resolveMetricsUrl()).toBe("/api/metrics");
  });

  it("refreshes quickly while generating and slowly while idle", () => {
    expect(metricsRefreshIntervalMs(true)).toBe(200);
    expect(metricsRefreshIntervalMs(false)).toBe(5000);
  });
});
