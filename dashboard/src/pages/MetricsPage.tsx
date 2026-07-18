import { useState } from "react";

import { MetricChart, type ChartSeries } from "../components/MetricChart";
import type { MetricsFrame } from "../metrics/prometheus";
import {
  buildAverageSeries,
  buildCounterRateSeries,
  buildGaugeSeries,
  buildHistogramSeries,
  buildPrefixHitRateSeries,
  metricLabelValues,
} from "../metrics/series";

type MetricsPageProps = { history: MetricsFrame[] };
type Category = "Serving" | "Scheduling" | "Cache" | "Reliability";
type ChartDefinition = {
  title: string;
  unit: string;
  decimals?: number;
  series: ChartSeries[];
};

const categories: Category[] = ["Serving", "Scheduling", "Cache", "Reliability"];
const colors = ["#3266d5", "#35a4c6", "#2b8a6e", "#c7832d", "#b55b75"];

function counterByLabel(
  history: MetricsFrame[],
  metric: string,
  label: string,
  excluded: string[] = [],
): ChartSeries[] {
  return metricLabelValues(history, metric, label)
    .filter((value) => !excluded.includes(value))
    .map((value, index) => ({
      name: value,
      color: colors[index % colors.length],
      data: buildCounterRateSeries(history, metric, { [label]: value }),
    }));
}

function chartsFor(category: Category, history: MetricsFrame[]): ChartDefinition[] {
  if (category === "Serving") {
    return [
      {
        title: "Inference requests",
        unit: "req/s",
        decimals: 2,
        series: [{ name: "Requests", color: colors[0], data: buildCounterRateSeries(history, "llm_requests_total") }],
      },
      {
        title: "Request duration",
        unit: "ms",
        series: [
          { name: "P50", color: colors[0], data: buildHistogramSeries(history, "llm_request_duration_seconds", 0.5, {}, 1000) },
          { name: "P99", color: colors[1], data: buildHistogramSeries(history, "llm_request_duration_seconds", 0.99, {}, 1000) },
        ],
      },
      {
        title: "TTFT",
        unit: "ms",
        series: [
          { name: "P50", color: colors[0], data: buildHistogramSeries(history, "llm_ttft_seconds", 0.5, {}, 1000) },
          { name: "P99", color: colors[1], data: buildHistogramSeries(history, "llm_ttft_seconds", 0.99, {}, 1000) },
        ],
      },
      {
        title: "TBT",
        unit: "ms",
        series: [
          { name: "P50", color: colors[0], data: buildHistogramSeries(history, "llm_tbt_seconds", 0.5, {}, 1000) },
          { name: "P99", color: colors[1], data: buildHistogramSeries(history, "llm_tbt_seconds", 0.99, {}, 1000) },
        ],
      },
      {
        title: "Executor execution time",
        unit: "ms",
        series: metricLabelValues(history, "llm_execution_seconds_count", "executor").map((executor, index) => ({
          name: executor,
          color: colors[index % colors.length],
          data: buildAverageSeries(history, "llm_execution_seconds", { executor }, 1000),
        })),
      },
    ];
  }

  if (category === "Scheduling") {
    return [
      {
        title: "Queue depth",
        unit: "items",
        series: [
          { name: "Prefill", color: colors[0], data: buildGaugeSeries(history, "llm_prefill_queue_length") },
          { name: "Decode", color: colors[1], data: buildGaugeSeries(history, "llm_decode_queue_length") },
        ],
      },
      {
        title: "Queue wait",
        unit: "ms",
        series: [
          { name: "P50", color: colors[0], data: buildHistogramSeries(history, "llm_queue_wait_seconds", 0.5, {}, 1000) },
          { name: "P99", color: colors[1], data: buildHistogramSeries(history, "llm_queue_wait_seconds", 0.99, {}, 1000) },
        ],
      },
      {
        title: "Batch size",
        unit: "sequences",
        series: [{ name: "Average", color: colors[0], data: buildAverageSeries(history, "llm_batch_size") }],
      },
      {
        title: "Batch items",
        unit: "items/s",
        series: counterByLabel(history, "llm_batch_items_total", "phase"),
      },
      {
        title: "Batches",
        unit: "batch/s",
        series: counterByLabel(history, "llm_batches_total", "executor"),
      },
      {
        title: "Inflight runtime work",
        unit: "count",
        series: [
          { name: "Requests", color: colors[0], data: buildGaugeSeries(history, "llm_active_requests") },
          { name: "Batches", color: colors[1], data: buildGaugeSeries(history, "llm_inflight_batches") },
        ],
      },
    ];
  }

  if (category === "Cache") {
    return [
      {
        title: "KV blocks",
        unit: "blocks",
        series: [
          { name: "Active", color: colors[0], data: buildGaugeSeries(history, "llm_kv_blocks", { state: "active" }) },
          { name: "Free", color: colors[2], data: buildGaugeSeries(history, "llm_kv_blocks", { state: "free" }) },
          { name: "Cached", color: colors[1], data: buildGaugeSeries(history, "llm_kv_blocks", { state: "cached" }) },
        ],
      },
      {
        title: "Prefix cache hit rate",
        unit: "%",
        series: [{ name: "Hit rate", color: colors[0], data: buildPrefixHitRateSeries(history) }],
      },
      {
        title: "Prefix tokens saved",
        unit: "tokens/s",
        series: [{ name: "Saved", color: colors[2], data: buildCounterRateSeries(history, "llm_prefix_cache_tokens_saved_total") }],
      },
      {
        title: "Cache events",
        unit: "events/s",
        series: [
          { name: "Allocation failures", color: "#c34e54", data: buildCounterRateSeries(history, "llm_kv_allocation_failures_total") },
          { name: "Evictions", color: colors[3], data: buildCounterRateSeries(history, "llm_prefix_cache_evictions_total") },
        ],
      },
    ];
  }

  return [
    {
      title: "Failed requests",
      unit: "errors/s",
      series: counterByLabel(history, "llm_requests_total", "status", ["ok"]),
    },
    {
      title: "Executor errors",
      unit: "errors/s",
      series: counterByLabel(history, "llm_executor_errors_total", "executor"),
    },
    {
      title: "Queue rejections",
      unit: "errors/s",
      series: [{ name: "Rejected", color: "#c34e54", data: buildCounterRateSeries(history, "llm_queue_rejected_total") }],
    },
    {
      title: "Allocation failures",
      unit: "errors/s",
      series: [{ name: "Failed", color: "#c34e54", data: buildCounterRateSeries(history, "llm_kv_allocation_failures_total") }],
    },
  ];
}

export function MetricsPage({ history }: MetricsPageProps) {
  const [category, setCategory] = useState<Category>("Serving");
  const charts = chartsFor(category, history);

  return (
    <section className="dashboard-page" data-testid="metrics-page">
      <div className="metric-tabs" role="tablist" aria-label="Metric category">
        {categories.map((item) => (
          <button
            aria-selected={category === item}
            className={category === item ? "is-active" : ""}
            key={item}
            onClick={() => setCategory(item)}
            role="tab"
            type="button"
          >
            {item}
          </button>
        ))}
      </div>
      <div className="metrics-grid">
        {charts.map((chart) => (
          <MetricChart key={chart.title} {...chart} />
        ))}
      </div>
    </section>
  );
}
