import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";

import { GetExecutorsResponseSchema } from "../gen/kvtide/v1/service_pb";
import { createRuntimeInventoryClient } from "./runtime";

describe("createRuntimeInventoryClient", () => {
  it("maps every executor runtime returned by the Engine", async () => {
    const rpc = vi.fn().mockResolvedValue(
      create(GetExecutorsResponseSchema, {
        executors: [
          {
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
          },
        ],
      }),
    );

    const executors = await createRuntimeInventoryClient(rpc).list();

    expect(executors).toEqual([
      expect.objectContaining({
        executorId: "executor-qwen",
        runtimeEpoch: 42,
        modelId: "Qwen/Qwen3-0.6B",
        kvCacheBytes: 512_000_000n,
      }),
    ]);
  });
});
