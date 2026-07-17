import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { GenerationClient } from "../playground/generation";
import type { MetricsClient, MetricsSnapshot } from "../playground/metrics";
import { DemoPage } from "./DemoPage";

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
  it("presents the runtime as topology, request, and metrics sections", async () => {
    const metrics: MetricsClient = {
      scrape: vi.fn().mockResolvedValue(snapshot()),
    };

    render(
      <DemoPage
        focusOnMount={false}
        client={{ generateStream: vi.fn() } as GenerationClient}
        metrics={metrics}
      />,
    );

    expect(screen.getByRole("heading", { name: "Live runtime" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Topology" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Send a request" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Runtime metrics" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /prefill|decode/i })).not.toBeInTheDocument();
    expect(await screen.findByText("0 P / 0 D")).toBeInTheDocument();
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
