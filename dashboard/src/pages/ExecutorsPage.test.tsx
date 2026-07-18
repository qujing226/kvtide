import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { RuntimeInfo } from "../api/runtime";
import { ExecutorsPage } from "./ExecutorsPage";

const runtime: RuntimeInfo = {
  executorId: "executor-qwen",
  runtimeEpoch: 42,
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

describe("ExecutorsPage", () => {
  it("shows runtime inventory without unsupported management actions", () => {
    render(<ExecutorsPage executors={[runtime]} />);

    expect(screen.getByText("executor-qwen")).toBeVisible();
    expect(screen.getByText("Qwen/Qwen3-0.6B")).toBeVisible();
    expect(screen.getByText("QWEN3 · CPU · 146 BLOCKS")).toBeVisible();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
