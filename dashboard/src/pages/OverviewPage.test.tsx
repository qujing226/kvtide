import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { RuntimeInfo } from "../api/runtime";
import { parsePrometheusText, type MetricsFrame } from "../metrics/prometheus";
import { OverviewPage } from "./OverviewPage";

const runtime: RuntimeInfo = {
  executorId: "executor-qwen",
  runtimeEpoch: 1,
  modelId: "Qwen/Qwen3-0.6B",
  modelType: "qwen3",
  dtype: "float32",
  deviceType: "cpu",
  tensorParallelSize: 1,
  blockSize: 16,
  numKvBlocks: 146,
  numHiddenLayers: 28,
  numKvHeads: 8,
  headDim: 128,
  totalMemoryBytes: 8_000_000_000n,
  availableMemoryBytes: 2_000_000_000n,
  kvCacheBytes: 512_000_000n,
};

const frame = (timestamp: number, text: string): MetricsFrame => ({
  timestamp,
  samples: parsePrometheusText(text),
});

describe("OverviewPage", () => {
  it("shows only the core runtime KPIs and client request trend", () => {
    const history = [
      frame(
        1_000,
        `llm_requests_total{status="ok",executor="executor-qwen"} 10
llm_active_requests 2
llm_prefix_cache_requests_total{status="hit"} 4
llm_prefix_cache_requests_total{status="miss"} 6
llm_ttft_seconds_bucket{le="0.1"} 1
llm_ttft_seconds_bucket{le="0.5"} 2
llm_ttft_seconds_bucket{le="+Inf"} 2`,
      ),
      frame(
        3_000,
        `llm_requests_total{status="ok",executor="executor-qwen"} 18
llm_active_requests 3
llm_prefix_cache_requests_total{status="hit"} 10
llm_prefix_cache_requests_total{status="miss"} 8
llm_ttft_seconds_bucket{le="0.1"} 2
llm_ttft_seconds_bucket{le="0.5"} 5
llm_ttft_seconds_bucket{le="+Inf"} 5`,
      ),
    ];

    render(<OverviewPage executors={[runtime]} history={history} />);

    const summary = within(
      screen.getByRole("region", { name: "Runtime summary" }),
    );
    expect(summary.getByText("Executors")).toBeVisible();
    expect(summary.getByText("Requests")).toBeVisible();
    expect(summary.getByText("TTFT P50")).toBeVisible();
    expect(summary.getByText("Active requests")).toBeVisible();
    expect(summary.getByText("Prefix hit rate")).toBeVisible();
    expect(screen.getByLabelText("Inference requests chart")).toBeVisible();
    expect(screen.queryByText("Prefill queue")).not.toBeInTheDocument();
    expect(screen.queryByText("Decode queue")).not.toBeInTheDocument();
    expect(screen.queryByText("KV block pool")).not.toBeInTheDocument();
    expect(screen.queryByText("Executor health")).not.toBeInTheDocument();
  });
});
