import { useEffect, useState } from "react";
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
import {
  calculateMetricsWindow,
  metricsRefreshIntervalMs,
  metricsClient,
  type MetricsClient,
  type MetricsSnapshot,
  type MetricsWindow,
} from "./metrics";

interface PlaygroundProps {
  language: Language;
  client?: GenerationClient;
  metrics?: MetricsClient;
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
  metrics = metricsClient,
}: PlaygroundProps) {
  const isZh = language === "zh";
  const [prompt, setPrompt] = useState(DEFAULT_PROMPT);
  const [userId] = useState(getPlaygroundUserID);
  const [generation, setGeneration] = useState(idleGenerationState);
  const [liveMetrics, setLiveMetrics] = useState<MetricsSnapshot | null>(null);
  const [metricsWindow, setMetricsWindow] = useState<MetricsWindow | null>(null);
  const [metricsStatus, setMetricsStatus] = useState<
    "connecting" | "live" | "unavailable"
  >("connecting");
  const [metricsUpdatedAt, setMetricsUpdatedAt] = useState<Date | null>(null);
  const isStreaming = generation.status === "streaming";
  const canSubmit = prompt.trim() !== "" && !isStreaming;

  useEffect(() => {
    let active = true;
    let scraping = false;

    const scrape = async () => {
      if (scraping) {
        return;
      }
      scraping = true;
      try {
        const snapshot = await metrics.scrape();
        if (active) {
          setLiveMetrics(snapshot);
          setMetricsStatus("live");
          setMetricsUpdatedAt(new Date());
        }
      } catch {
        if (active) {
          setMetricsStatus("unavailable");
        }
      } finally {
        scraping = false;
      }
    };

    void scrape();
    const interval = window.setInterval(() => {
      void scrape();
    }, metricsRefreshIntervalMs(isStreaming));

    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [isStreaming, metrics]);

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

    setMetricsWindow(null);
    let before = liveMetrics;
    try {
      before = await metrics.scrape();
      setLiveMetrics(before);
      setMetricsStatus("live");
      setMetricsUpdatedAt(new Date());
    } catch {
      setMetricsStatus("unavailable");
    }

    await generateResponse({
      client,
      prompt: submittedPrompt,
      requestId: newRequestID(),
      userId,
      now: () => performance.now(),
      onState: setGeneration,
    });

    try {
      const after = await metrics.scrape();
      setLiveMetrics(after);
      setMetricsStatus("live");
      setMetricsUpdatedAt(new Date());
      if (before) {
        setMetricsWindow(calculateMetricsWindow(before, after));
      }
    } catch {
      setMetricsStatus("unavailable");
    }
  };

  const metricsStatusLabel = {
    connecting: isZh ? "连接中" : "CONNECTING",
    live: "LIVE",
    unavailable: isZh ? "不可用" : "UNAVAILABLE",
  }[metricsStatus];
  const prefixCacheLabel = {
    hit: "HIT",
    miss: "MISS",
    mixed: "MIXED",
    none: "—",
  }[metricsWindow?.prefixCache ?? "none"];
  const windowErrors = metricsWindow
    ? [
        metricsWindow.errors.queueRejected > 0
          ? `${isZh ? "队列拒绝" : "QUEUE REJECTED"} ${metricsWindow.errors.queueRejected}`
          : "",
        metricsWindow.errors.executorErrors > 0
          ? `${isZh ? "执行器错误" : "EXECUTOR ERRORS"} ${metricsWindow.errors.executorErrors}`
          : "",
        metricsWindow.errors.allocationFailures > 0
          ? `${isZh ? "KV 分配失败" : "KV ALLOCATION FAILURES"} ${metricsWindow.errors.allocationFailures}`
          : "",
        metricsWindow.errors.cacheEvictions > 0
          ? `${isZh ? "CACHE 淘汰" : "CACHE EVICTIONS"} ${metricsWindow.errors.cacheEvictions}`
          : "",
      ].filter(Boolean)
    : [];
  const formatMilliseconds = (value: number | null | undefined) =>
    value == null ? "—" : `${Math.round(value)} ms`;

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

      <section
        className="control-plane-metrics"
        aria-label={isZh ? "控制平面指标" : "control plane metrics"}
      >
        <header>
          <div>
            <span className="section-kicker">
              {isZh ? "控制平面指标" : "CONTROL PLANE METRICS"}
            </span>
            <small>
              {isZh
                ? "直接抓取 Prometheus exposition"
                : "DIRECT PROMETHEUS SCRAPE"}
            </small>
          </div>
          <div className={`scrape-status scrape-${metricsStatus}`}>
            <span />
            <strong>{metricsStatusLabel}</strong>
            <time>
              {metricsUpdatedAt
                ? metricsUpdatedAt.toLocaleTimeString([], {
                    hour: "2-digit",
                    minute: "2-digit",
                    second: "2-digit",
                  })
                : "—"}
            </time>
          </div>
        </header>

        <div className="live-metrics-grid">
          <article>
            <span>{isZh ? "队列" : "QUEUES"}</span>
            <strong>
              {liveMetrics
                ? `${liveMetrics.runtime.prefillQueue} P / ${liveMetrics.runtime.decodeQueue} D`
                : "—"}
            </strong>
            <small>PREFILL / DECODE</small>
          </article>
          <article>
            <span>{isZh ? "执行中" : "INFLIGHT"}</span>
            <strong>
              {liveMetrics
                ? `${liveMetrics.runtime.activeRequests} R / ${liveMetrics.runtime.inflightBatches} B`
                : "—"}
            </strong>
            <small>REQUESTS / BATCHES</small>
          </article>
          <article>
            <span>KV BLOCKS</span>
            <strong>
              {liveMetrics
                ? `${liveMetrics.runtime.kvActive} A / ${liveMetrics.runtime.kvFree} F / ${liveMetrics.runtime.kvCached} C`
                : "—"}
            </strong>
            <small>ACTIVE / FREE / CACHED</small>
          </article>
        </div>

        <div className="metrics-window-heading">
          <span>{isZh ? "请求窗口" : "REQUEST WINDOW"}</span>
          <small>
            {isZh
              ? "生成期间的全局指标增量"
              : "GLOBAL DELTAS DURING GENERATION"}
          </small>
        </div>
        <div className="window-metrics-grid">
          <article>
            <span>PREFIX CACHE</span>
            <strong>{prefixCacheLabel}</strong>
          </article>
          <article>
            <span>{isZh ? "节省 TOKEN" : "TOKENS SAVED"}</span>
            <strong>
              {metricsWindow ? `+${metricsWindow.tokensSaved}` : "—"}
            </strong>
          </article>
          <article>
            <span>BATCH</span>
            <strong>
              {metricsWindow
                ? `${metricsWindow.batches} · AVG ${
                    metricsWindow.averageBatchSize?.toFixed(1) ?? "—"
                  }`
                : "—"}
            </strong>
          </article>
          <article>
            <span>{isZh ? "任务" : "WORK ITEMS"}</span>
            <strong>
              {metricsWindow
                ? `${metricsWindow.prefillItems} P / ${metricsWindow.decodeItems} D`
                : "—"}
            </strong>
          </article>
          <article>
            <span>{isZh ? "排队等待" : "QUEUE WAIT"}</span>
            <strong>
              {formatMilliseconds(metricsWindow?.averageQueueWaitMs)}
            </strong>
          </article>
          <article>
            <span>{isZh ? "执行" : "EXECUTION"}</span>
            <strong>
              {formatMilliseconds(metricsWindow?.averageExecutionMs)}
            </strong>
          </article>
          <article>
            <span>TBT</span>
            <strong>{formatMilliseconds(metricsWindow?.averageTbtMs)}</strong>
          </article>
        </div>

        {windowErrors.length > 0 && (
          <div className="metrics-alert" role="status">
            {windowErrors.join(" · ")}
          </div>
        )}
      </section>
    </main>
  );
}
