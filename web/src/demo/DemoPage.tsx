import { useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { PageTransition } from "../site/PageTransition";
import { SnapScroller } from "../site/SnapScroller";
import { generationClient } from "./client";
import {
  generateResponse,
  idleGenerationState,
  type GenerationClient,
} from "./generation";
import { getDemoUserID } from "./identity";
import {
  calculateMetricsWindow,
  metricsClient,
  metricsRefreshIntervalMs,
  type MetricsClient,
  type MetricsSnapshot,
  type MetricsWindow,
} from "./metrics";
import {
  runtimeInventoryClient,
  type RuntimeInfo,
  type RuntimeInventoryClient,
} from "./runtimes";
import { RuntimeTopology } from "./RuntimeTopology";
import "./demo.css";

type DemoPageProps = {
  focusOnMount: boolean;
  client?: GenerationClient;
  metrics?: MetricsClient;
  runtimes?: RuntimeInventoryClient;
};

export const DEFAULT_DEMO_PROMPT =
  "Explain how continuous batching and paged KV cache management improve inference throughput while preserving request isolation across repeated prompts in a production LLM serving system with predictable latency and efficient memory reuse.";

const RUNTIME_DISCOVERY_INTERVAL_MS = 10_000;

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

function formatBytes(bytes: bigint) {
  if (bytes === 0n) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = Number(bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 ? value.toFixed(1) : value.toFixed(2)} ${units[unit]}`;
}

function queueInsight(metrics: MetricsSnapshot | null) {
  if (!metrics) return "Waiting for a runtime sample.";
  const { prefillQueue, decodeQueue } = metrics.runtime;
  if (prefillQueue + decodeQueue === 0) return "Queues are idle.";
  if (decodeQueue > prefillQueue) return "Decode work currently leads the queue.";
  if (prefillQueue > decodeQueue) return "Prefill work currently leads the queue.";
  return "Prefill and decode pressure are balanced.";
}

function inflightInsight(metrics: MetricsSnapshot | null) {
  if (!metrics) return "Waiting for a runtime sample.";
  const { activeRequests, inflightBatches } = metrics.runtime;
  if (activeRequests === 0) return "No request is executing.";
  return `${activeRequests} active request${activeRequests === 1 ? "" : "s"} across ${inflightBatches} batch${inflightBatches === 1 ? "" : "es"}.`;
}

function kvInsight(metrics: MetricsSnapshot | null) {
  if (!metrics) return "Waiting for a runtime sample.";
  const { kvActive, kvFree, kvCached } = metrics.runtime;
  const total = kvActive + kvFree + kvCached;
  if (total === 0) return "KV capacity is not reported.";
  return `${Math.round((kvFree / total) * 100)}% KV headroom remains available.`;
}

function RuntimeDetails({ runtime, onClose }: { runtime: RuntimeInfo; onClose(): void }) {
  return (
    <aside className="runtime-details" data-snap-ignore aria-label={`${runtime.executorId} runtime details`}>
      <div className="runtime-details-header">
        <div>
          <span>Executor runtime</span>
          <h3>{runtime.executorId}</h3>
        </div>
        <button type="button" onClick={onClose} aria-label="Close runtime details">
          Close
        </button>
      </div>
      <dl>
        <div><dt>Model</dt><dd>{runtime.modelId}</dd></div>
        <div><dt>Runtime epoch</dt><dd>{runtime.runtimeEpoch}</dd></div>
        <div><dt>Device</dt><dd>{runtime.deviceType} · {runtime.dtype}</dd></div>
        <div><dt>Tensor parallel</dt><dd>{runtime.tensorParallelSize}</dd></div>
        <div><dt>KV layout</dt><dd>{runtime.numKvBlocks} blocks · {runtime.blockSize} tokens</dd></div>
        <div><dt>Model shape</dt><dd>{runtime.numHiddenLayers} layers · {runtime.numKvHeads} KV heads · {runtime.headDim} dim</dd></div>
        <div><dt>KV cache</dt><dd>{formatBytes(runtime.kvCacheBytes)}</dd></div>
        <div><dt>Memory available</dt><dd>{formatBytes(runtime.availableMemoryBytes)} / {formatBytes(runtime.totalMemoryBytes)}</dd></div>
      </dl>
    </aside>
  );
}

export function DemoPage({
  focusOnMount,
  client = generationClient,
  metrics = metricsClient,
  runtimes = runtimeInventoryClient,
}: DemoPageProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);
  const [prompt, setPrompt] = useState(DEFAULT_DEMO_PROMPT);
  const [userId] = useState(getDemoUserID);
  const [generation, setGeneration] = useState(idleGenerationState);
  const [liveMetrics, setLiveMetrics] = useState<MetricsSnapshot | null>(null);
  const [metricsWindow, setMetricsWindow] = useState<MetricsWindow | null>(null);
  const [metricsState, setMetricsState] = useState<"connecting" | "live" | "offline">("connecting");
  const [runtimeList, setRuntimeList] = useState<RuntimeInfo[]>([]);
  const [runtimeState, setRuntimeState] = useState<"connecting" | "live" | "offline">("connecting");
  const [selectedExecutor, setSelectedExecutor] = useState<string | null>(null);
  const isRunning = generation.status === "streaming";
  const canSubmit = prompt.trim() !== "" && !isRunning;
  const selectedRuntime = runtimeList.find((runtime) => runtime.executorId === selectedExecutor) ?? null;

  useEffect(() => {
    document.title = "Demo | KVTide";
    if (focusOnMount) headingRef.current?.focus();
  }, [focusOnMount]);

  useEffect(() => {
    let mounted = true;
    let discovering = false;

    const discover = async () => {
      if (discovering) return;
      discovering = true;
      try {
        const next = await runtimes.list();
        if (mounted) {
          setRuntimeList(next);
          setRuntimeState("live");
        }
      } catch {
        if (mounted) setRuntimeState("offline");
      } finally {
        discovering = false;
      }
    };

    void discover();
    const timer = window.setInterval(
      () => void discover(),
      RUNTIME_DISCOVERY_INTERVAL_MS,
    );
    return () => {
      mounted = false;
      window.clearInterval(timer);
    };
  }, [runtimes]);

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
    <PageTransition>
      <SnapScroller className="demo-scroll-container">
        <section className="demo-screen demo-topology-screen" data-snap-screen aria-labelledby="demo-topology-title">
          <div className="demo-screen-shell">
            <div className="demo-screen-heading" data-reveal="1">
              <div>
                <span className="demo-index">01</span>
                <h1 id="demo-topology-title" ref={headingRef} tabIndex={-1}>Topology</h1>
              </div>
              <span className={`demo-connection-state is-${runtimeState}`}>{runtimeState}</span>
            </div>
            <div className={`demo-topology-layout${selectedRuntime ? " has-details" : ""}`} data-reveal="2">
              <RuntimeTopology
                active={isRunning}
                runtimes={runtimeList}
                selectedExecutor={selectedExecutor}
                onSelectExecutor={setSelectedExecutor}
              />
              {selectedRuntime && (
                <RuntimeDetails runtime={selectedRuntime} onClose={() => setSelectedExecutor(null)} />
              )}
            </div>
          </div>
        </section>

        <section className="demo-screen demo-request-screen" data-snap-screen aria-labelledby="demo-request-title">
          <div className="demo-screen-shell demo-request-shell">
            <div className="demo-screen-heading" data-reveal="1">
              <div>
                <span className="demo-index">02</span>
                <h2 id="demo-request-title">Send a request</h2>
              </div>
            </div>
            <div className="demo-request-workspace" data-reveal="2">
              <form
                className="demo-request-form"
                onSubmit={(event) => {
                  event.preventDefault();
                  void submit();
                }}
              >
                <label htmlFor="demo-prompt">Prompt</label>
                <textarea
                  data-snap-ignore
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
                <div className="demo-response-body" data-snap-ignore>
                  {generation.text ? (
                    <Markdown remarkPlugins={[remarkGfm]}>{generation.text}</Markdown>
                  ) : (
                    <p className="demo-empty">Model output will stream here.</p>
                  )}
                  {generation.error && <p className="demo-error">{generation.error}</p>}
                  {isRunning && <span className="demo-cursor" aria-hidden="true" />}
                </div>
              </section>
            </div>
            <div className="demo-request-stats" data-reveal="3" aria-label="Request measurements">
              <div><span>TTFT</span><strong>{generation.ttftMs == null ? "—" : `${generation.ttftMs} ms`}</strong></div>
              <div><span>Output</span><strong>{generation.outputTokens == null ? "—" : `${generation.outputTokens} tokens`}</strong></div>
              <div><span>Status</span><strong>{statusLabel(generation.status)}</strong></div>
            </div>
          </div>
        </section>

        <section className="demo-screen demo-metrics-screen" data-snap-screen aria-labelledby="demo-metrics-title">
          <div className="demo-screen-shell demo-metrics-shell">
            <div className="demo-screen-heading" data-reveal="1">
              <div>
                <span className="demo-index">03</span>
                <h2 id="demo-metrics-title">Runtime metrics</h2>
              </div>
              <span className={`demo-connection-state is-${metricsState}`}>{metricsState}</span>
            </div>
            <div className="demo-runtime-grid" data-reveal="2">
              <article>
                <span>Queues</span>
                <strong>{liveMetrics ? `${liveMetrics.runtime.prefillQueue} P / ${liveMetrics.runtime.decodeQueue} D` : "—"}</strong>
                <small>Prefill / Decode</small>
                <p>{queueInsight(liveMetrics)}</p>
              </article>
              <article>
                <span>Inflight</span>
                <strong>{liveMetrics ? `${liveMetrics.runtime.activeRequests} R / ${liveMetrics.runtime.inflightBatches} B` : "—"}</strong>
                <small>Requests / Batches</small>
                <p>{inflightInsight(liveMetrics)}</p>
              </article>
              <article>
                <span>KV blocks</span>
                <strong>{liveMetrics ? `${liveMetrics.runtime.kvActive} A / ${liveMetrics.runtime.kvFree} F / ${liveMetrics.runtime.kvCached} C` : "—"}</strong>
                <small>Active / Free / Cached</small>
                <p>{kvInsight(liveMetrics)}</p>
              </article>
            </div>
            <div className="demo-window-grid" data-reveal="3" aria-label="Latest request metrics">
              <div><span>Prefix cache</span><strong>{prefixCache}</strong><small>Latest request</small></div>
              <div><span>Tokens saved</span><strong>{metricsWindow ? metricsWindow.tokensSaved : "—"}</strong><small>Prefix reuse</small></div>
              <div><span>Batches</span><strong>{metricsWindow ? metricsWindow.batches : "—"}</strong><small>Latest request</small></div>
            </div>
          </div>
        </section>
      </SnapScroller>
    </PageTransition>
  );
}
