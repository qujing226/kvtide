import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { RuntimeInfo } from "./runtimes";
import { RuntimeTopology } from "./RuntimeTopology";

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

describe("RuntimeTopology", () => {
  it("keeps the topology cards focused on engine and executor identity", () => {
    render(
      <RuntimeTopology
        active={false}
        runtimes={[runtime]}
        selectedExecutor={null}
        onSelectExecutor={vi.fn()}
      />,
    );

    expect(screen.getByTestId("runtime-topology")).toHaveAttribute("data-snap-ignore");
    expect(screen.getByText("Engine")).toBeInTheDocument();
    expect(screen.getByText("Executor")).toBeInTheDocument();
    expect(screen.getByText("executor-qwen")).toBeInTheDocument();
    expect(screen.queryByText("CONTROL PLANE")).not.toBeInTheDocument();
    expect(screen.queryByText("MODEL RUNTIME")).not.toBeInTheDocument();
    expect(screen.queryByText("schedule · blocks · stream")).not.toBeInTheDocument();
    expect(document.querySelectorAll(".topology-status-dot")).toHaveLength(2);
  });

  it("zooms, resets, and exposes the executor as an interactive node", async () => {
    const user = userEvent.setup();
    const onSelectExecutor = vi.fn();
    render(
      <RuntimeTopology
        active={false}
        runtimes={[runtime]}
        selectedExecutor={null}
        onSelectExecutor={onSelectExecutor}
      />,
    );

    const topology = screen.getByTestId("runtime-topology");
    expect(topology).toHaveAttribute("data-scale", "1.00");

    await user.click(screen.getByRole("button", { name: "Zoom in" }));
    expect(topology).toHaveAttribute("data-scale", "1.15");

    await user.click(screen.getByRole("button", { name: "Reset topology view" }));
    expect(topology).toHaveAttribute("data-scale", "1.00");

    await user.click(screen.getByRole("button", { name: "Inspect executor-qwen" }));
    expect(onSelectExecutor).toHaveBeenCalledWith("executor-qwen");
  });
});
