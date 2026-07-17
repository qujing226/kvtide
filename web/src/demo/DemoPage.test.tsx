import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { GenerationClient } from "./generation";
import type { MetricsClient, MetricsSnapshot } from "./metrics";
import type { RuntimeInfo, RuntimeInventoryClient } from "./runtimes";
import { DemoPage } from "./DemoPage";

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

function runtimeClient(): RuntimeInventoryClient {
  return { list: vi.fn().mockResolvedValue([runtime]) };
}

function snapshot(
  runtime: Partial<MetricsSnapshot["runtime"]> = {},
): MetricsSnapshot {
  return {
    runtime: {
      prefillQueue: 0,
      decodeQueue: 0,
      activeRequests: 0,
      inflightBatches: 0,
      kvActive: 0,
      kvFree: 1024,
      kvCached: 0,
      ...runtime,
    },
    counters: {
      prefixHits: 0,
      prefixMisses: 0,
      tokensSaved: 0,
      batches: 0,
      batchSizeSum: 0,
      batchSizeCount: 0,
      prefillItems: 0,
      decodeItems: 0,
      queueWaitSum: 0,
      queueWaitCount: 0,
      executionSum: 0,
      executionCount: 0,
      tbtSum: 0,
      tbtCount: 0,
      queueRejected: 0,
      executorErrors: 0,
      allocationFailures: 0,
      cacheEvictions: 0,
    },
  };
}

describe("DemoPage", () => {
  it("retries runtime discovery every ten seconds after the backend starts late", async () => {
    vi.useFakeTimers();
    try {
      const list = vi
        .fn()
        .mockRejectedValueOnce(new Error("offline"))
        .mockResolvedValueOnce([runtime]);

      render(
        <DemoPage
          focusOnMount={false}
          client={{ generateStream: vi.fn() } as GenerationClient}
          metrics={{ scrape: vi.fn().mockResolvedValue(snapshot()) }}
          runtimes={{ list }}
        />,
      );

      await act(async () => Promise.resolve());
      expect(list).toHaveBeenCalledOnce();
      expect(screen.queryByRole("button", { name: "Inspect executor-qwen" })).not.toBeInTheDocument();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(9_999);
      });
      expect(list).toHaveBeenCalledOnce();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      expect(list).toHaveBeenCalledTimes(2);
      expect(screen.getByRole("button", { name: "Inspect executor-qwen" })).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("presents topology, request, and metrics as three viewport screens", async () => {
    const metrics: MetricsClient = {
      scrape: vi.fn().mockResolvedValue(snapshot()),
    };

    render(
      <DemoPage
        focusOnMount={false}
        client={{ generateStream: vi.fn() } as GenerationClient}
        metrics={metrics}
        runtimes={runtimeClient()}
      />,
    );

    expect(screen.queryByRole("heading", { name: "Live runtime" })).not.toBeInTheDocument();
    expect(document.querySelectorAll(".demo-screen")).toHaveLength(3);
    expect(screen.getByRole("heading", { name: "Topology" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Send a request" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Runtime metrics" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Zoom in" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /prefill|decode/i })).not.toBeInTheDocument();
    expect(await screen.findByText("0 P / 0 D")).toBeInTheDocument();
    expect(screen.getByText("Queues are idle.")).toBeInTheDocument();

    expect(screen.queryByText("Qwen/Qwen3-0.6B")).not.toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", { name: "Inspect executor-qwen" }),
    );
    expect(await screen.findByText("Qwen/Qwen3-0.6B")).toBeInTheDocument();
  });

  it("streams markdown through one send action and marks the topology active", async () => {
    const user = userEvent.setup();
    let release: () => void = () => {};
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    const client: GenerationClient = {
      generateStream: () => ({
        async *[Symbol.asyncIterator]() {
          await pending;
          yield { deltaText: "## Paged cache\n\n", done: false };
          yield { deltaText: "Reuses **KV blocks**.", done: true, outputTokens: 5 };
        },
      }),
    };

    render(
      <DemoPage
        focusOnMount={false}
        client={client}
        metrics={{ scrape: vi.fn().mockResolvedValue(snapshot()) }}
        runtimes={runtimeClient()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Send" }));
    expect(screen.getByRole("button", { name: "Running" })).toBeDisabled();
    expect(screen.getByTestId("runtime-topology")).toHaveAttribute(
      "data-active",
      "true",
    );

    release();

    expect(
      await screen.findByRole("heading", { name: "Paged cache" }),
    ).toBeVisible();
    expect(
      screen.getAllByText("KV blocks").some((element) => element.tagName === "STRONG"),
    ).toBe(true);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Send" })).toBeEnabled();
    });
    expect(screen.getByText("5 tokens")).toBeVisible();
    expect(screen.getByTestId("runtime-topology")).toHaveAttribute(
      "data-active",
      "false",
    );
  });
});
