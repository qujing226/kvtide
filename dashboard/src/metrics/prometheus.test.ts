import { describe, expect, it } from "vitest";

import {
  counterRate,
  histogramQuantile,
  parsePrometheusText,
  sumMetric,
  type MetricsFrame,
} from "./prometheus";

const frame = (timestamp: number, text: string): MetricsFrame => ({
  timestamp,
  samples: parsePrometheusText(text),
});

describe("parsePrometheusText", () => {
  it("keeps labels needed for executor and status filtering", () => {
    const samples = parsePrometheusText(`
# HELP llm_requests_total Total number of requests
llm_requests_total{status="ok",executor="executor-a"} 12
llm_requests_total{status="error",executor="executor-b"} 3
`);

    expect(samples).toEqual([
      {
        name: "llm_requests_total",
        labels: { status: "ok", executor: "executor-a" },
        value: 12,
      },
      {
        name: "llm_requests_total",
        labels: { status: "error", executor: "executor-b" },
        value: 3,
      },
    ]);
  });

  it("ignores invalid and non-finite samples", () => {
    const samples = parsePrometheusText(`
not prometheus
llm_active_requests NaN
llm_active_requests 4
`);

    expect(samples).toEqual([
      { name: "llm_active_requests", labels: {}, value: 4 },
    ]);
  });
});

describe("metric calculations", () => {
  it("sums only samples matching the requested labels", () => {
    const samples = parsePrometheusText(`
llm_requests_total{status="ok",executor="executor-a"} 12
llm_requests_total{status="ok",executor="executor-b"} 8
llm_requests_total{status="error",executor="executor-a"} 2
`);

    expect(sumMetric(samples, "llm_requests_total", { status: "ok" })).toBe(20);
    expect(
      sumMetric(samples, "llm_requests_total", { executor: "executor-a" }),
    ).toBe(14);
  });

  it("calculates a per-second counter rate and protects against resets", () => {
    const before = frame(1_000, "llm_requests_total 10");
    const after = frame(3_000, "llm_requests_total 18");
    const reset = frame(5_000, "llm_requests_total 2");

    expect(counterRate(before, after, "llm_requests_total")).toBe(4);
    expect(counterRate(after, reset, "llm_requests_total")).toBe(0);
  });

  it("derives a quantile from cumulative histogram bucket deltas", () => {
    const before = frame(
      1_000,
      `
llm_ttft_seconds_bucket{le="0.1"} 4
llm_ttft_seconds_bucket{le="0.5"} 8
llm_ttft_seconds_bucket{le="1"} 10
llm_ttft_seconds_bucket{le="+Inf"} 10
`,
    );
    const after = frame(
      3_000,
      `
llm_ttft_seconds_bucket{le="0.1"} 5
llm_ttft_seconds_bucket{le="0.5"} 11
llm_ttft_seconds_bucket{le="1"} 14
llm_ttft_seconds_bucket{le="+Inf"} 14
`,
    );

    expect(histogramQuantile(before, after, "llm_ttft_seconds", 0.5)).toBeCloseTo(
      0.3,
    );
  });
});
