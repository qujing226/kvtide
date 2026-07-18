import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import {
  AdminService,
  GetExecutorsRequestSchema,
  type GetExecutorsResponse,
} from "../gen/kvtide/v1/service_pb";

export type RuntimeInfo = {
  executorId: string;
  runtimeEpoch: number;
  modelId: string;
  modelType: string;
  dtype: string;
  deviceType: string;
  tensorParallelSize: number;
  blockSize: number;
  numKvBlocks: number;
  numHiddenLayers: number;
  numKvHeads: number;
  headDim: number;
  totalMemoryBytes: bigint;
  availableMemoryBytes: bigint;
  kvCacheBytes: bigint;
};

export type RuntimeInventoryClient = {
  list(): Promise<RuntimeInfo[]>;
};

type GetExecutorsRpc = () => Promise<GetExecutorsResponse>;

export function createRuntimeInventoryClient(
  getExecutors: GetExecutorsRpc,
): RuntimeInventoryClient {
  return {
    async list() {
      const response = await getExecutors();
      return response.executors.map((runtime) => ({
        executorId: runtime.executorId,
        runtimeEpoch: runtime.runtimeEpoch,
        modelId: runtime.modelId,
        modelType: runtime.modelType,
        dtype: runtime.dtype,
        deviceType: runtime.deviceType,
        tensorParallelSize: runtime.tensorParallelSize,
        blockSize: runtime.blockSize,
        numKvBlocks: runtime.numKvBlocks,
        numHiddenLayers: runtime.numHiddenLayers,
        numKvHeads: runtime.numKvHeads,
        headDim: runtime.headDim,
        totalMemoryBytes: runtime.totalMemoryBytes,
        availableMemoryBytes: runtime.availableMemoryBytes,
        kvCacheBytes: runtime.kvCacheBytes,
      }));
    },
  };
}

const transport = createConnectTransport({ baseUrl: window.location.origin });
const adminClient = createClient(AdminService, transport);

export const runtimeInventoryClient = createRuntimeInventoryClient(() =>
  adminClient.getExecutors(create(GetExecutorsRequestSchema)),
);
