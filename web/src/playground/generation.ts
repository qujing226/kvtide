export type GenerationStatus = "idle" | "streaming" | "completed" | "failed";

export interface GenerationRequest {
  requestId: string;
  userId: string;
  model: string;
  prompt: string;
  maxTokens: number;
  timeoutMs: number;
}

export interface GenerationChunk {
  deltaText: string;
  done: boolean;
  finishReason?: "stop" | "length" | "timeout" | "error" | "unspecified";
  outputTokens?: number;
  errorMessage?: string;
}

export interface GenerationClient {
  generateStream(request: GenerationRequest): AsyncIterable<GenerationChunk>;
}

export interface GenerationState {
  status: GenerationStatus;
  text: string;
  requestId: string;
  ttftMs: number | null;
  outputTokens: number | null;
  error: string | null;
}

interface GenerateResponseOptions {
  client: GenerationClient;
  prompt: string;
  requestId: string;
  userId: string;
  now: () => number;
  onState: (state: GenerationState) => void;
}

export const idleGenerationState: GenerationState = {
  status: "idle",
  text: "",
  requestId: "",
  ttftMs: null,
  outputTokens: null,
  error: null,
};

export async function generateResponse({
  client,
  prompt,
  requestId,
  userId,
  now,
  onState,
}: GenerateResponseOptions): Promise<GenerationState> {
  const startedAt = now();
  let state: GenerationState = {
    ...idleGenerationState,
    status: "streaming",
    requestId,
  };
  onState(state);

  try {
    for await (const chunk of client.generateStream({
      requestId,
      userId,
      model: "Qwen/Qwen3-0.6B",
      prompt,
      maxTokens: 128,
      timeoutMs: 30_000,
    })) {
      if (chunk.errorMessage) {
        throw new Error(chunk.errorMessage);
      }

      const firstText = state.ttftMs === null && chunk.deltaText !== "";
      const finishError =
        chunk.finishReason === "timeout"
          ? "The request timed out."
          : chunk.finishReason === "error"
            ? "The request failed during generation."
            : null;
      state = {
        ...state,
        status: finishError
          ? "failed"
          : chunk.done
            ? "completed"
            : "streaming",
        text: state.text + chunk.deltaText,
        ttftMs: firstText ? Math.max(0, Math.round(now() - startedAt)) : state.ttftMs,
        outputTokens: chunk.done
          ? (chunk.outputTokens ?? state.outputTokens)
          : state.outputTokens,
        error: finishError,
      };
      onState(state);

      if (chunk.done) {
        return state;
      }
    }

    state = {
      ...state,
      status: "failed",
      error: "The response stream ended before completion.",
    };
  } catch (error) {
    state = {
      ...state,
      status: "failed",
      error: error instanceof Error ? error.message : "The request failed.",
    };
  }

  onState(state);
  return state;
}
