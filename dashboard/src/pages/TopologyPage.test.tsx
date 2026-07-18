import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { RuntimeInfo } from "../api/runtime";
import { TopologyPage } from "./TopologyPage";

const runtime = {
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
} satisfies RuntimeInfo;

describe("TopologyPage", () => {
  it("uses minimal Engine and executor node labels", () => {
    render(<TopologyPage executors={[runtime]} />);

    expect(screen.getByText("Engine")).toBeInTheDocument();
    expect(screen.getByText("executor-qwen")).toBeInTheDocument();
    expect(screen.getByText("QWEN3 · CPU · 146 BLOCKS")).toBeInTheDocument();
    expect(screen.queryByText(/control plane/i)).not.toBeInTheDocument();
  });
});
