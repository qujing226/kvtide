import { describe, expect, it } from "vitest";

import { parsePrometheusText, type MetricsFrame } from "./prometheus";
import {
  buildAverageSeries,
  buildCounterRateSeries,
  buildGaugeSeries,
  buildHistogramSeries,
  buildPrefixHitRateSeries,
  metricLabelValues,
} from "./series";

const frame = (timestamp: number, text: string): MetricsFrame => ({
  timestamp,
  samples: parsePrometheusText(text),
});

describe("metric series", () => {
  it("builds gauge and labeled counter-rate points", () => {
    const history = [
      frame(
        1_000,
        `llm_active_requests 2
llm_requests_total{status="ok"} 10`,
      ),
      frame(
        3_000,
        `llm_active_requests 5
llm_requests_total{status="ok"} 18`,
      ),
    ];

    expect(buildGaugeSeries(history, "llm_active_requests")).toEqual([
      { timestamp: 1_000, value: 2 },
      { timestamp: 3_000, value: 5 },
    ]);
    expect(
      buildCounterRateSeries(history, "llm_requests_total", { status: "ok" }),
    ).toEqual([{ timestamp: 3_000, value: 4 }]);
  });

  it("builds histogram quantiles in display units", () => {
    const history = [
      frame(
        1_000,
        `llm_ttft_seconds_bucket{le="0.1"} 1
llm_ttft_seconds_bucket{le="0.5"} 2
llm_ttft_seconds_bucket{le="+Inf"} 2`,
      ),
      frame(
        3_000,
        `llm_ttft_seconds_bucket{le="0.1"} 2
llm_ttft_seconds_bucket{le="0.5"} 5
llm_ttft_seconds_bucket{le="+Inf"} 5`,
      ),
    ];

    expect(buildHistogramSeries(history, "llm_ttft_seconds", 0.5, {}, 1000)).toEqual([
      { timestamp: 3_000, value: 200 },
    ]);
  });

  it("calculates prefix hit rate from hit and miss deltas", () => {
    const history = [
      frame(
        1_000,
        `llm_prefix_cache_requests_total{status="hit"} 4
llm_prefix_cache_requests_total{status="miss"} 6`,
      ),
      frame(
        3_000,
        `llm_prefix_cache_requests_total{status="hit"} 10
llm_prefix_cache_requests_total{status="miss"} 8`,
      ),
    ];

    expect(buildPrefixHitRateSeries(history)).toEqual([
      { timestamp: 3_000, value: 75 },
    ]);
  });

  it("calculates an average from histogram sum and count deltas", () => {
    const history = [
      frame(1_000, "llm_batch_size_sum 10\nllm_batch_size_count 2"),
      frame(3_000, "llm_batch_size_sum 22\nllm_batch_size_count 5"),
    ];

    expect(buildAverageSeries(history, "llm_batch_size")).toEqual([
      { timestamp: 3_000, value: 4 },
    ]);
  });

  it("discovers available labels for per-executor series", () => {
    const history = [
      frame(
        1_000,
        `llm_executor_errors_total{executor="executor-b"} 1
llm_executor_errors_total{executor="executor-a"} 2`,
      ),
    ];

    expect(
      metricLabelValues(history, "llm_executor_errors_total", "executor"),
    ).toEqual(["executor-a", "executor-b"]);
  });
});
