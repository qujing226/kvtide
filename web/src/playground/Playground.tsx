import { useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import type { Language } from "../app/content";
import { generationClient } from "./client";
import {
  generateResponse,
  idleGenerationState,
  type GenerationClient,
} from "./generation";
import { getPlaygroundUserID } from "./identity";

interface PlaygroundProps {
  language: Language;
  client?: GenerationClient;
}

export const DEFAULT_PROMPT =
  "Explain how continuous batching and paged KV cache management improve inference throughput while preserving request isolation across repeated prompts in a production LLM serving system with predictable latency and efficient memory reuse.";

function newRequestID() {
  return typeof crypto.randomUUID === "function"
    ? `web-${crypto.randomUUID()}`
    : `web-${Date.now()}`;
}

export function Playground({
  language,
  client = generationClient,
}: PlaygroundProps) {
  const isZh = language === "zh";
  const [prompt, setPrompt] = useState(DEFAULT_PROMPT);
  const [userId] = useState(getPlaygroundUserID);
  const [generation, setGeneration] = useState(idleGenerationState);
  const isStreaming = generation.status === "streaming";
  const canSubmit = prompt.trim() !== "" && !isStreaming;

  const status = {
    idle: isZh ? "等待请求" : "Idle",
    streaming: isZh ? "生成中" : "Streaming",
    completed: isZh ? "已完成" : "Completed",
    failed: isZh ? "请求失败" : "Failed",
  }[generation.status];

  const submit = async () => {
    const submittedPrompt = prompt.trim();
    if (submittedPrompt === "" || isStreaming) {
      return;
    }

    await generateResponse({
      client,
      prompt: submittedPrompt,
      requestId: newRequestID(),
      userId,
      now: () => performance.now(),
      onState: setGeneration,
    });
  };

  return (
    <main className="page playground-page">
      <section className="playground-workbench">
        <form
          className="request-panel"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <label className="prompt-field">
            <textarea
              aria-label={isZh ? "提示词" : "Prompt"}
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={
                isZh
                  ? "例如：解释 continuous batching 如何提升推理吞吐量。"
                  : "For example: Explain how continuous batching improves inference throughput."
              }
              disabled={isStreaming}
            />
          </label>
          <button className="generate-button" type="submit" disabled={!canSubmit}>
            {isStreaming
              ? isZh
                ? "生成中"
                : "Generating"
              : isZh
                ? "生成响应"
                : "Generate response"}
          </button>
        </form>

        <section
          className={`response-panel response-${generation.status}`}
          aria-live="polite"
        >
          <header>
            <div className="section-kicker">
              {isZh ? "流式输出" : "STREAMING OUTPUT"}
            </div>
            <strong>{status}</strong>
          </header>
          <div className="response-body">
            {generation.error ? (
              <p className="response-error">{generation.error}</p>
            ) : generation.text ? (
              <Markdown remarkPlugins={[remarkGfm]}>
                {generation.text}
              </Markdown>
            ) : (
              <p className="response-empty">
                {isZh
                  ? "响应内容将在请求开始后显示在这里。"
                  : "Generated output will appear here after the request starts."}
              </p>
            )}
            {isStreaming && <span className="stream-cursor" aria-hidden="true" />}
          </div>
          <footer>
            <span>
              {generation.requestId
                ? `REQUEST · ${generation.requestId}`
                : isZh
                  ? "尚无请求"
                  : "NO REQUEST"}
            </span>
          </footer>
        </section>
      </section>

      <section
        className="playground-metrics"
        aria-label={isZh ? "请求指标" : "request measurements"}
      >
        <article>
          <span>TTFT</span>
          <strong>
            {generation.ttftMs === null ? "—" : `${generation.ttftMs} ms`}
          </strong>
          <small>{isZh ? "浏览器观测" : "browser observed"}</small>
        </article>
        <article>
          <span>{isZh ? "输出" : "OUTPUT"}</span>
          <strong>
            {generation.outputTokens === null
              ? "—"
              : `${generation.outputTokens} tokens`}
          </strong>
          <small>{isZh ? "服务端 usage" : "server usage"}</small>
        </article>
        <article>
          <span>{isZh ? "状态" : "STATUS"}</span>
          <strong>{status}</strong>
          <small>GenerateStream</small>
        </article>
      </section>
    </main>
  );
}
