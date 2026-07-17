export interface RuntimeMetrics {
  prefillQueue: number;
  decodeQueue: number;
  activeRequests: number;
  inflightBatches: number;
  kvActive: number;
  kvFree: number;
  kvCached: number;
}

export interface CounterMetrics {
  prefixHits: number;
  prefixMisses: number;
  tokensSaved: number;
  batches: number;
  batchSizeSum: number;
  batchSizeCount: number;
  prefillItems: number;
  decodeItems: number;
  queueWaitSum: number;
  queueWaitCount: number;
  executionSum: number;
  executionCount: number;
  tbtSum: number;
  tbtCount: number;
  queueRejected: number;
  executorErrors: number;
  allocationFailures: number;
  cacheEvictions: number;
}

export interface MetricsSnapshot {
  runtime: RuntimeMetrics;
  counters: CounterMetrics;
}

export type PrefixCacheResult = "hit" | "miss" | "mixed" | "none";

export interface MetricsWindow {
  prefixCache: PrefixCacheResult;
  tokensSaved: number;
  batches: number;
  averageBatchSize: number | null;
  prefillItems: number;
  decodeItems: number;
  averageQueueWaitMs: number | null;
  averageExecutionMs: number | null;
  averageTbtMs: number | null;
  errors: {
    queueRejected: number;
    executorErrors: number;
    allocationFailures: number;
    cacheEvictions: number;
  };
}

export interface MetricsClient {
  scrape(): Promise<MetricsSnapshot>;
}

export function metricsRefreshIntervalMs(isStreaming: boolean): number {
  return isStreaming ? 200 : 5000;
}

interface MetricSample {
  name: string;
  labels: Record<string, string>;
  value: number;
}

const samplePattern =
  /^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+([^\s]+)(?:\s+\d+)?$/;
const labelPattern = /([a-zA-Z_][a-zA-Z0-9_]*)="((?:\\.|[^"])*)"/g;

function parseLabels(rawLabels = ""): Record<string, string> {
  const labels: Record<string, string> = {};

  for (const match of rawLabels.matchAll(labelPattern)) {
    const [, key, rawValue] = match;
    if (!key || rawValue === undefined) {
      continue;
    }
    labels[key] = rawValue
      .replaceAll("\\n", "\n")
      .replaceAll('\\"', '"')
      .replaceAll("\\\\", "\\");
  }

  return labels;
}

function parseSamples(text: string): MetricSample[] {
  const samples: MetricSample[] = [];

  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line === "" || line.startsWith("#")) {
      continue;
    }

    const match = line.match(samplePattern);
    if (!match) {
      continue;
    }

    const [, name, rawLabels, rawValue] = match;
    const value = Number(rawValue);
    if (!name || !Number.isFinite(value)) {
      continue;
    }

    samples.push({
      name,
      labels: parseLabels(rawLabels),
      value,
    });
  }

  return samples;
}

function sum(
  samples: MetricSample[],
  name: string,
  labels?: Record<string, string>,
): number {
  return samples
    .filter(
      (sample) =>
        sample.name === name &&
        Object.entries(labels ?? {}).every(
          ([key, value]) => sample.labels[key] === value,
        ),
    )
    .reduce((total, sample) => total + sample.value, 0);
}

export function parseMetricsSnapshot(text: string): MetricsSnapshot {
  const samples = parseSamples(text);

  return {
    runtime: {
      prefillQueue: sum(samples, "llm_prefill_queue_length"),
      decodeQueue: sum(samples, "llm_decode_queue_length"),
      activeRequests: sum(samples, "llm_active_requests"),
      inflightBatches: sum(samples, "llm_inflight_batches"),
      kvActive: sum(samples, "llm_kv_blocks", { state: "active" }),
      kvFree: sum(samples, "llm_kv_blocks", { state: "free" }),
      kvCached: sum(samples, "llm_kv_blocks", { state: "cached" }),
    },
    counters: {
      prefixHits: sum(samples, "llm_prefix_cache_requests_total", {
        status: "hit",
      }),
      prefixMisses: sum(samples, "llm_prefix_cache_requests_total", {
        status: "miss",
      }),
      tokensSaved: sum(samples, "llm_prefix_cache_tokens_saved_total"),
      batches: sum(samples, "llm_batches_total"),
      batchSizeSum: sum(samples, "llm_batch_size_sum"),
      batchSizeCount: sum(samples, "llm_batch_size_count"),
      prefillItems: sum(samples, "llm_batch_items_total", {
        phase: "prefill",
      }),
      decodeItems: sum(samples, "llm_batch_items_total", {
        phase: "decode",
      }),
      queueWaitSum: sum(samples, "llm_queue_wait_seconds_sum"),
      queueWaitCount: sum(samples, "llm_queue_wait_seconds_count"),
      executionSum: sum(samples, "llm_execution_seconds_sum"),
      executionCount: sum(samples, "llm_execution_seconds_count"),
      tbtSum: sum(samples, "llm_tbt_seconds_sum"),
      tbtCount: sum(samples, "llm_tbt_seconds_count"),
      queueRejected: sum(samples, "llm_queue_rejected_total"),
      executorErrors: sum(samples, "llm_executor_errors_total"),
      allocationFailures: sum(
        samples,
        "llm_kv_allocation_failures_total",
      ),
      cacheEvictions: sum(samples, "llm_prefix_cache_evictions_total"),
    },
  };
}

function counterDelta(before: number, after: number): number {
  return after >= before ? after - before : 0;
}

function averageDelta(
  beforeSum: number,
  beforeCount: number,
  afterSum: number,
  afterCount: number,
  scale = 1,
): number | null {
  const count = counterDelta(beforeCount, afterCount);
  const total = counterDelta(beforeSum, afterSum);
  return count > 0 ? (total / count) * scale : null;
}

export function calculateMetricsWindow(
  before: MetricsSnapshot,
  after: MetricsSnapshot,
): MetricsWindow {
  const hitCount = counterDelta(
    before.counters.prefixHits,
    after.counters.prefixHits,
  );
  const missCount = counterDelta(
    before.counters.prefixMisses,
    after.counters.prefixMisses,
  );
  const prefixCache =
    hitCount > 0 && missCount > 0
      ? "mixed"
      : hitCount > 0
        ? "hit"
        : missCount > 0
          ? "miss"
          : "none";

  return {
    prefixCache,
    tokensSaved: counterDelta(
      before.counters.tokensSaved,
      after.counters.tokensSaved,
    ),
    batches: counterDelta(before.counters.batches, after.counters.batches),
    averageBatchSize: averageDelta(
      before.counters.batchSizeSum,
      before.counters.batchSizeCount,
      after.counters.batchSizeSum,
      after.counters.batchSizeCount,
    ),
    prefillItems: counterDelta(
      before.counters.prefillItems,
      after.counters.prefillItems,
    ),
    decodeItems: counterDelta(
      before.counters.decodeItems,
      after.counters.decodeItems,
    ),
    averageQueueWaitMs: averageDelta(
      before.counters.queueWaitSum,
      before.counters.queueWaitCount,
      after.counters.queueWaitSum,
      after.counters.queueWaitCount,
      1000,
    ),
    averageExecutionMs: averageDelta(
      before.counters.executionSum,
      before.counters.executionCount,
      after.counters.executionSum,
      after.counters.executionCount,
      1000,
    ),
    averageTbtMs: averageDelta(
      before.counters.tbtSum,
      before.counters.tbtCount,
      after.counters.tbtSum,
      after.counters.tbtCount,
      1000,
    ),
    errors: {
      queueRejected: counterDelta(
        before.counters.queueRejected,
        after.counters.queueRejected,
      ),
      executorErrors: counterDelta(
        before.counters.executorErrors,
        after.counters.executorErrors,
      ),
      allocationFailures: counterDelta(
        before.counters.allocationFailures,
        after.counters.allocationFailures,
      ),
      cacheEvictions: counterDelta(
        before.counters.cacheEvictions,
        after.counters.cacheEvictions,
      ),
    },
  };
}

export function resolveMetricsUrl(): string {
  return "/api/metrics";
}

export function createMetricsClient(
  url = resolveMetricsUrl(),
  fetcher: typeof fetch = fetch,
): MetricsClient {
  return {
    async scrape() {
      const response = await fetcher(url, {
        headers: {
          Accept: "text/plain",
        },
      });
      if (!response.ok) {
        throw new Error(`metrics scrape failed: ${response.status}`);
      }
      return parseMetricsSnapshot(await response.text());
    },
  };
}

export const metricsClient = createMetricsClient();
