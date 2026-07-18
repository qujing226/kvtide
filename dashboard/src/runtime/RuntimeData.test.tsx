import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { MetricsClient } from "../api/metrics";
import type { RuntimeInventoryClient } from "../api/runtime";
import { RuntimeDataProvider, useRuntimeData } from "./RuntimeData";

function Probe() {
  const runtime = useRuntimeData();
  return (
    <output>
      {runtime.connection}:{runtime.executors.length}:{runtime.history.length}
    </output>
  );
}

describe("RuntimeDataProvider", () => {
  it("publishes the first metrics and executor snapshots", async () => {
    const metrics: MetricsClient = {
      scrape: vi.fn().mockResolvedValue({ timestamp: 42, samples: [] }),
    };
    const inventory: RuntimeInventoryClient = {
      list: vi.fn().mockResolvedValue([
        {
          executorId: "executor-a",
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
        },
      ]),
    };

    render(
      <RuntimeDataProvider metrics={metrics} inventory={inventory}>
        <Probe />
      </RuntimeDataProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("connected:1:1")).toBeVisible();
    });
  });
});
