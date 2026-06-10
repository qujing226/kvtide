import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { FinishReason } from "../gen/mini_llm_serve/v1/core_pb";
import {
  GenerateRequestSchema,
  InferenceService,
  type GenerateRequest,
  type GenerateResponseChunk,
} from "../gen/mini_llm_serve/v1/service_pb";
import type {
  GenerationChunk,
  GenerationClient,
  GenerationRequest,
} from "./generation";

type GenerateStreamRpc = (
  request: GenerateRequest,
) => AsyncIterable<GenerateResponseChunk>;

function finishReasonName(reason: FinishReason): GenerationChunk["finishReason"] {
  switch (reason) {
    case FinishReason.STOP:
      return "stop";
    case FinishReason.LENGTH:
      return "length";
    case FinishReason.TIMEOUT:
      return "timeout";
    case FinishReason.ERROR:
      return "error";
    default:
      return "unspecified";
  }
}

export function createGenerationClient(
  generateStreamRpc: GenerateStreamRpc,
): GenerationClient {
  return {
    async *generateStream(request: GenerationRequest) {
      const rpcRequest = create(GenerateRequestSchema, request);

      for await (const chunk of generateStreamRpc(rpcRequest)) {
        yield {
          deltaText: chunk.deltaText,
          done: chunk.done,
          finishReason: finishReasonName(chunk.finishReason),
          outputTokens: chunk.usage?.outputTokens,
          errorMessage: chunk.errorMessage,
        };
      }
    },
  };
}

const transport = createConnectTransport({
  baseUrl: window.location.origin,
});
const inferenceClient = createClient(InferenceService, transport);

export const generationClient = createGenerationClient((request) =>
  inferenceClient.generateStream(request),
);
