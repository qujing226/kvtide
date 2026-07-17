import { describe, expect, it, vi } from "vitest";

import {
  generateResponse,
  type GenerationChunk,
  type GenerationClient,
  type GenerationState,
} from "./generation";

function stream(chunks: GenerationChunk[]): AsyncIterable<GenerationChunk> {
  return {
    async *[Symbol.asyncIterator]() {
      for (const chunk of chunks) {
        yield chunk;
      }
    },
  };
}

describe("generateResponse", () => {
  it("streams text and records TTFT and completed usage", async () => {
    const generateStream = vi.fn(() =>
      stream([
        { deltaText: "Continuous ", done: false },
        {
          deltaText: "batching.",
          done: true,
          outputTokens: 2,
        },
      ]),
    );
    const client: GenerationClient = { generateStream };
    const states: GenerationState[] = [];
    const times = [100, 142];

    const result = await generateResponse({
      client,
      prompt: "Explain continuous batching.",
      requestId: "req-test",
      userId: "web-user",
      now: () => times.shift() ?? 142,
      onState: (state) => states.push(state),
    });

    expect(generateStream).toHaveBeenCalledWith({
      requestId: "req-test",
      userId: "web-user",
      model: "Qwen/Qwen3-0.6B",
      prompt: "Explain continuous batching.",
      maxTokens: 128,
      timeoutMs: 30_000,
    });
    expect(states.map((state) => state.status)).toEqual([
      "streaming",
      "streaming",
      "completed",
    ]);
    expect(result).toMatchObject({
      status: "completed",
      text: "Continuous batching.",
      requestId: "req-test",
      ttftMs: 42,
      outputTokens: 2,
      error: null,
    });
  });

  it("returns a failed state when the stream ends without completion", async () => {
    const client: GenerationClient = {
      generateStream: () => stream([{ deltaText: "partial", done: false }]),
    };

    const result = await generateResponse({
      client,
      prompt: "Prompt",
      requestId: "req-incomplete",
      userId: "web-user",
      now: () => 10,
      onState: () => undefined,
    });

    expect(result.status).toBe("failed");
    expect(result.text).toBe("partial");
    expect(result.error).toMatch(/ended before completion/i);
  });

  it("preserves partial output when the client throws", async () => {
    const client: GenerationClient = {
      generateStream: () => ({
        async *[Symbol.asyncIterator]() {
          yield { deltaText: "partial", done: false };
          throw new Error("backend unavailable");
        },
      }),
    };

    const result = await generateResponse({
      client,
      prompt: "Prompt",
      requestId: "req-failed",
      userId: "web-user",
      now: () => 10,
      onState: () => undefined,
    });

    expect(result).toMatchObject({
      status: "failed",
      text: "partial",
      error: "backend unavailable",
    });
  });

  it("treats timeout completion as a failed request", async () => {
    const client: GenerationClient = {
      generateStream: () =>
        stream([
          {
            deltaText: "partial",
            done: true,
            finishReason: "timeout",
            outputTokens: 1,
          },
        ]),
    };

    const result = await generateResponse({
      client,
      prompt: "Prompt",
      requestId: "req-timeout",
      userId: "web-user",
      now: () => 10,
      onState: () => undefined,
    });

    expect(result).toMatchObject({
      status: "failed",
      text: "partial",
      outputTokens: 1,
      error: "The request timed out.",
    });
  });
});
