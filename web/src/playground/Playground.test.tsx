import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { GenerationClient } from "./generation";
import type { MetricsClient, MetricsSnapshot } from "./metrics";
import { DEFAULT_PROMPT, Playground } from "./Playground";

function metricsSnapshot(
  runtime: Partial<MetricsSnapshot["runtime"]> = {},
  counters: Partial<MetricsSnapshot["counters"]> = {},
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
      ...counters,
    },
  };
}

describe("Playground", () => {
  it("starts with a cacheable two-block prompt", () => {
    render(
      <Playground
        language="en"
        client={{ generateStream: vi.fn() } as GenerationClient}
      />,
    );

    expect(DEFAULT_PROMPT.trim().split(/\s+/)).toHaveLength(32);
    expect(screen.getByLabelText(/prompt/i)).toHaveValue(DEFAULT_PROMPT);
  });

  it("does not submit a cleared prompt", async () => {
    const user = userEvent.setup();
    const generateStream = vi.fn();
    render(
      <Playground
        language="en"
        client={{ generateStream } as GenerationClient}
      />,
    );

    const submit = screen.getByRole("button", { name: /generate response/i });
    await user.clear(screen.getByLabelText(/prompt/i));
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText(/prompt/i), "   ");
    expect(submit).toBeDisabled();
    expect(generateStream).not.toHaveBeenCalled();
  });

  it("streams a response and reports request measurements", async () => {
    const user = userEvent.setup();
    let releaseStream: () => void = () => {};
    const waiting = new Promise<void>((resolve) => {
      releaseStream = resolve;
    });
    const client: GenerationClient = {
      generateStream: () => ({
        async *[Symbol.asyncIterator]() {
          await waiting;
          yield { deltaText: "# Continuous batching\n\n", done: false };
          yield {
            deltaText: "- Preserves **throughput**.",
            done: true,
            outputTokens: 4,
          };
        },
      }),
    };
    render(<Playground language="en" client={client} />);

    await user.clear(screen.getByLabelText(/prompt/i));
    await user.type(
      screen.getByLabelText(/prompt/i),
      "Explain continuous batching.",
    );
    await user.click(
      screen.getByRole("button", { name: /generate response/i }),
    );

    expect(
      screen.getByRole("button", { name: /generating/i }),
    ).toBeDisabled();

    releaseStream();

    expect(
      await screen.findByRole("heading", { name: "Continuous batching" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("list")).toBeInTheDocument();
    expect(screen.getByText("throughput").tagName).toBe("STRONG");
    await waitFor(() => {
      expect(screen.getAllByText("Completed").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("4 tokens")).toBeInTheDocument();
    expect(screen.getByText(/\d+ ms/)).toBeInTheDocument();
  });

  it("shows live control-plane metrics and request-window deltas", async () => {
    const user = userEvent.setup();
    const before = metricsSnapshot(
      {
        prefillQueue: 2,
        decodeQueue: 3,
        activeRequests: 1,
        inflightBatches: 1,
        kvActive: 8,
        kvFree: 1000,
        kvCached: 16,
      },
      {
        prefixHits: 4,
        prefixMisses: 6,
        tokensSaved: 64,
        batches: 10,
        batchSizeSum: 18,
        batchSizeCount: 10,
        prefillItems: 5,
        decodeItems: 13,
        queueWaitSum: 0.2,
        queueWaitCount: 10,
        executionSum: 1.5,
        executionCount: 10,
        tbtSum: 0.8,
        tbtCount: 8,
      },
    );
    const after = metricsSnapshot(
      {
        decodeQueue: 1,
        kvActive: 4,
        kvFree: 1004,
        kvCached: 24,
      },
      {
        prefixHits: 5,
        prefixMisses: 6,
        tokensSaved: 96,
        batches: 14,
        batchSizeSum: 26,
        batchSizeCount: 14,
        prefillItems: 6,
        decodeItems: 20,
        queueWaitSum: 0.24,
        queueWaitCount: 14,
        executionSum: 1.94,
        executionCount: 14,
        tbtSum: 1.1,
        tbtCount: 11,
        queueRejected: 1,
        allocationFailures: 2,
        cacheEvictions: 1,
      },
    );
    const scrape = vi
      .fn<MetricsClient["scrape"]>()
      .mockResolvedValueOnce(before)
      .mockResolvedValueOnce(before)
      .mockResolvedValueOnce(after);
    const generationClient: GenerationClient = {
      generateStream: () => ({
        async *[Symbol.asyncIterator]() {
          yield {
            deltaText: "complete",
            done: true,
            outputTokens: 1,
          };
        },
      }),
    };

    render(
      <Playground
        language="en"
        client={generationClient}
        metrics={{ scrape }}
      />,
    );

    expect(await screen.findByText("2 P / 3 D")).toBeInTheDocument();
    expect(screen.getByText("8 A / 1000 F / 16 C")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /generate response/i }),
    );

    expect(await screen.findByText("HIT")).toBeInTheDocument();
    expect(screen.getByText("+32")).toBeInTheDocument();
    expect(screen.getByText("4 · AVG 2.0")).toBeInTheDocument();
    expect(screen.getByText("1 P / 7 D")).toBeInTheDocument();
    expect(screen.getByText("10 ms")).toBeInTheDocument();
    expect(screen.getByText("110 ms")).toBeInTheDocument();
    expect(screen.getByText("100 ms")).toBeInTheDocument();
    expect(screen.getByText(/QUEUE REJECTED 1/)).toBeInTheDocument();
    expect(screen.getByText(/KV ALLOCATION FAILURES 2/)).toBeInTheDocument();
  });

  it("keeps generation available when metrics scraping fails", async () => {
    const user = userEvent.setup();
    const metrics: MetricsClient = {
      scrape: vi.fn().mockRejectedValue(new Error("offline")),
    };
    const generationClient: GenerationClient = {
      generateStream: () => ({
        async *[Symbol.asyncIterator]() {
          yield { deltaText: "complete", done: true, outputTokens: 1 };
        },
      }),
    };

    render(
      <Playground
        language="en"
        client={generationClient}
        metrics={metrics}
      />,
    );

    expect(await screen.findByText("UNAVAILABLE")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: /generate response/i }),
    );
    expect(await screen.findByText("complete")).toBeInTheDocument();
  });
});
