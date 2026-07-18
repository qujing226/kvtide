import type { RuntimeInfo } from "../api/runtime";
import { KpiCard } from "../components/KpiCard";
import { MetricChart } from "../components/MetricChart";
import type { MetricsFrame } from "../metrics/prometheus";
import {
  buildCounterRateSeries,
  buildGaugeSeries,
  buildHistogramSeries,
  buildPrefixHitRateSeries,
  type TimeSeriesPoint,
} from "../metrics/series";

type OverviewPageProps = {
  executors: RuntimeInfo[];
  history: MetricsFrame[];
};

function latest(series: TimeSeriesPoint[]): number | null {
  return series.at(-1)?.value ?? null;
}

function value(value: number | null, decimals = 1): string {
  return value === null ? "—" : value.toFixed(decimals);
}

export function OverviewPage({ executors, history }: OverviewPageProps) {
  const requests = buildCounterRateSeries(history, "llm_requests_total");
  const ttft = buildHistogramSeries(history, "llm_ttft_seconds", 0.5, {}, 1000);
  const activeRequests = buildGaugeSeries(history, "llm_active_requests");
  const prefixHitRate = buildPrefixHitRateSeries(history);

  return (
    <section className="dashboard-page overview-page" data-testid="overview-page">
      <div className="kpi-grid" role="region" aria-label="Runtime summary">
        <KpiCard
          detail="registered runtimes"
          label="Executors"
          value={String(executors.length)}
        />
        <KpiCard
          detail="client inference traffic"
          label="Requests"
          unit="req/s"
          value={value(latest(requests), 2)}
        />
        <KpiCard
          detail="time to first token"
          label="TTFT P50"
          unit="ms"
          value={value(latest(ttft), 0)}
        />
        <KpiCard
          detail="currently in the runtime"
          label="Active requests"
          value={value(latest(activeRequests), 0)}
        />
        <KpiCard
          detail="over the current sample window"
          label="Prefix hit rate"
          unit="%"
          value={value(latest(prefixHitRate), 1)}
        />
      </div>
      <MetricChart
        decimals={2}
        series={[{ name: "Requests", color: "#3266d5", data: requests }]}
        title="Inference requests"
        unit="req/s"
      />
    </section>
  );
}
