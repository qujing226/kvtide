import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";

import { GetRuntimesResponseSchema } from "../gen/kvtide/v1/service_pb";
import { createRuntimeInventoryClient } from "./runtimes";

describe("createRuntimeInventoryClient", () => {
  it("maps control-plane runtime snapshots to the topology boundary", async () => {
    const rpc = vi.fn().mockResolvedValue(
      create(GetRuntimesResponseSchema, {
        runtimes: [
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

    const runtimes = await createRuntimeInventoryClient(rpc).list();

    expect(rpc).toHaveBeenCalledOnce();
    expect(runtimes).toEqual([
      expect.objectContaining({
        executorId: "executor-qwen",
        runtimeEpoch: 42,
        modelId: "Qwen/Qwen3-0.6B",
        deviceType: "cpu",
        kvCacheBytes: 512_000_000n,
      }),
    ]);
  });
});
