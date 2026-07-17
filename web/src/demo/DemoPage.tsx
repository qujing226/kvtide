import { useEffect, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { RoutePage } from "../site/RoutePage";
import { generationClient } from "./client";
import {
  generateResponse,
  idleGenerationState,
  type GenerationClient,
} from "./generation";
import {
  calculateMetricsWindow,
  metricsClient,
  metricsRefreshIntervalMs,
  type MetricsClient,
  type MetricsSnapshot,
  type MetricsWindow,
} from "./metrics";
import { RuntimeTopology } from "./RuntimeTopology";
import { getDemoUserID } from "./identity";
import "./demo.css";

type DemoPageProps = {
  focusOnMount: boolean;
  client?: GenerationClient;
  metrics?: MetricsClient;
};

export const DEFAULT_DEMO_PROMPT =
  "Explain how continuous batching and paged KV cache management improve inference throughput while preserving request isolation across repeated prompts in a production LLM serving system with predictable latency and efficient memory reuse.";

function newRequestID() {
  return typeof crypto.randomUUID === "function"
    ? `web-${crypto.randomUUID()}`
    : `web-${Date.now()}`;
}

function statusLabel(status: typeof idleGenerationState.status) {
  return {
    idle: "Idle",
    streaming: "Streaming",
    completed: "Completed",
    failed: "Failed",
  }[status];
}

export function DemoPage({
  focusOnMount,
  client = generationClient,
  metrics = metricsClient,
}: DemoPageProps) {
  const [prompt, setPrompt] = useState(DEFAULT_DEMO_PROMPT);
  const [userId] = useState(getDemoUserID);
  const [generation, setGeneration] = useState(idleGenerationState);
  const [liveMetrics, setLiveMetrics] = useState<MetricsSnapshot | null>(null);
  const [metricsWindow, setMetricsWindow] = useState<MetricsWindow | null>(null);
  const [metricsState, setMetricsState] = useState<"connecting" | "live" | "offline">(
    "connecting",
  );
  const isRunning = generation.status === "streaming";
  const canSubmit = prompt.trim() !== "" && !isRunning;

  useEffect(() => {
    let mounted = true;
    let scraping = false;

    const scrape = async () => {
      if (scraping) return;
      scraping = true;
      try {
        const next = await metrics.scrape();
        if (mounted) {
          setLiveMetrics(next);
          setMetricsState("live");
        }
      } catch {
        if (mounted) setMetricsState("offline");
      } finally {
        scraping = false;
      }
    };

    void scrape();
    const timer = window.setInterval(() => void scrape(), metricsRefreshIntervalMs(isRunning));
    return () => {
      mounted = false;
      window.clearInterval(timer);
    };
  }, [isRunning, metrics]);

  const submit = async () => {
    const submittedPrompt = prompt.trim();
    if (!submittedPrompt || isRunning) return;

    setMetricsWindow(null);
    let before = liveMetrics;
    try {
      before = await metrics.scrape();
      setLiveMetrics(before);
      setMetricsState("live");
    } catch {
      setMetricsState("offline");
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
      setMetricsState("live");
      if (before) setMetricsWindow(calculateMetricsWindow(before, after));
    } catch {
      setMetricsState("offline");
    }
  };

  const prefixCache = {
    hit: "Hit",
    miss: "Miss",
    mixed: "Mixed",
    none: "—",
  }[metricsWindow?.prefixCache ?? "none"];

  return (
    <RoutePage title="Live runtime" focusOnMount={focusOnMount}>
      <p className="demo-intro">
        Observe one request moving through the KVTide control plane and executor.
      </p>

      <div className="demo-story">
        <aside className="demo-sticky" aria-label="Runtime topology">
          <RuntimeTopology active={isRunning} />
        </aside>

        <div className="demo-sections">
          <section className="demo-section demo-topology-copy">
            <span className="demo-index">01</span>
            <h2>Topology</h2>
            <p>
              The browser opens a streamed request to the Go control plane. The scheduler
              binds the sequence to the Qwen executor that owns its KV blocks.
            </p>
          </section>

          <section className="demo-section demo-request-section">
            <span className="demo-index">02</span>
            <h2>Send a request</h2>
            <form
              className="demo-request-form"
              onSubmit={(event) => {
                event.preventDefault();
                void submit();
              }}
            >
              <label htmlFor="demo-prompt">Prompt</label>
              <textarea
                id="demo-prompt"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                disabled={isRunning}
              />
              <button type="submit" disabled={!canSubmit}>
                {isRunning ? "Running" : "Send"}
              </button>
            </form>

            <section className="demo-response" aria-live="polite">
              <div className="demo-response-header">
                <span>Response</span>
                <strong>{statusLabel(generation.status)}</strong>
              </div>
              <div className="demo-response-body">
                {generation.text ? (
                  <Markdown remarkPlugins={[remarkGfm]}>{generation.text}</Markdown>
                ) : (
                  <p className="demo-empty">Model output will stream here.</p>
                )}
                {generation.error && <p className="demo-error">{generation.error}</p>}
                {isRunning && <span className="demo-cursor" aria-hidden="true" />}
              </div>
            </section>

            <div className="demo-request-stats" aria-label="Request measurements">
              <div><span>TTFT</span><strong>{generation.ttftMs == null ? "—" : `${generation.ttftMs} ms`}</strong></div>
              <div><span>Output</span><strong>{generation.outputTokens == null ? "—" : `${generation.outputTokens} tokens`}</strong></div>
              <div><span>Status</span><strong>{statusLabel(generation.status)}</strong></div>
            </div>
          </section>

          <section className="demo-section demo-metrics-section">
            <div className="demo-metrics-heading">
              <div>
                <span className="demo-index">03</span>
                <h2>Runtime metrics</h2>
              </div>
              <span className={`demo-metrics-state is-${metricsState}`}>{metricsState}</span>
            </div>
            <div className="demo-runtime-grid">
              <article>
                <span>Queues</span>
                <strong>{liveMetrics ? `${liveMetrics.runtime.prefillQueue} P / ${liveMetrics.runtime.decodeQueue} D` : "—"}</strong>
                <small>Prefill / Decode</small>
              </article>
              <article>
                <span>Inflight</span>
                <strong>{liveMetrics ? `${liveMetrics.runtime.activeRequests} R / ${liveMetrics.runtime.inflightBatches} B` : "—"}</strong>
                <small>Requests / Batches</small>
              </article>
              <article>
                <span>KV blocks</span>
                <strong>{liveMetrics ? `${liveMetrics.runtime.kvActive} A / ${liveMetrics.runtime.kvFree} F / ${liveMetrics.runtime.kvCached} C` : "—"}</strong>
                <small>Active / Free / Cached</small>
              </article>
            </div>
            <div className="demo-window-grid" aria-label="Latest request metrics">
              <div><span>Prefix cache</span><strong>{prefixCache}</strong></div>
              <div><span>Tokens saved</span><strong>{metricsWindow ? metricsWindow.tokensSaved : "—"}</strong></div>
              <div><span>Batches</span><strong>{metricsWindow ? metricsWindow.batches : "—"}</strong></div>
            </div>
          </section>
        </div>
      </div>
    </RoutePage>
  );
}
