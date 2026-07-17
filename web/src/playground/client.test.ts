import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";

import { FinishReason } from "../gen/kvtide/v1/core_pb";
import {
  GenerateResponseChunkSchema,
  type GenerateRequest,
} from "../gen/kvtide/v1/service_pb";
import { createGenerationClient, resolveInferenceBaseUrl } from "./client";

describe("resolveInferenceBaseUrl", () => {
  it("targets the backend port on the current browser host", () => {
    expect(
      resolveInferenceBaseUrl({
        protocol: "http:",
        hostname: "127.0.0.1",
      }),
    ).toBe("http://127.0.0.1:8800");
  });
});

describe("createGenerationClient", () => {
  it("maps generated stream chunks to the playground boundary", async () => {
    const rpc = vi.fn((_request: GenerateRequest) => ({
      async *[Symbol.asyncIterator]() {
        yield create(GenerateResponseChunkSchema, {
          deltaText: "hello",
          done: true,
          finishReason: FinishReason.LENGTH,
          usage: { outputTokens: 1 },
        });
      },
    }));
    const client = createGenerationClient(rpc);

    const chunks = [];
    for await (const chunk of client.generateStream({
      requestId: "req-test",
      userId: "web-user",
      model: "mock",
      prompt: "Hello",
      maxTokens: 128,
      timeoutMs: 30_000,
    })) {
      chunks.push(chunk);
    }

    expect(rpc).toHaveBeenCalledWith(
      expect.objectContaining({
        requestId: "req-test",
        userId: "web-user",
        modelId: "mock",
        prompt: "Hello",
        maxTokens: 128,
        timeoutMs: 30_000,
      }),
    );
    expect(chunks).toEqual([
      {
        deltaText: "hello",
        done: true,
        finishReason: "length",
        outputTokens: 1,
        errorMessage: "",
      },
    ]);
  });
});
