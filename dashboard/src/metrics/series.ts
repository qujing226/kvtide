import {
  counterRate,
  histogramQuantile,
  sumMetric,
  type MetricLabels,
  type MetricsFrame,
} from "./prometheus";

export type TimeSeriesPoint = {
  timestamp: number;
  value: number;
};

export function buildGaugeSeries(
  history: MetricsFrame[],
  name: string,
  labels: MetricLabels = {},
): TimeSeriesPoint[] {
  return history.map((frame) => ({
    timestamp: frame.timestamp,
    value: sumMetric(frame.samples, name, labels),
  }));
}

export function buildCounterRateSeries(
  history: MetricsFrame[],
  name: string,
  labels: MetricLabels = {},
): TimeSeriesPoint[] {
  return history.slice(1).map((frame, index) => ({
    timestamp: frame.timestamp,
    value: counterRate(history[index], frame, name, labels),
  }));
}

export function buildHistogramSeries(
  history: MetricsFrame[],
  name: string,
  quantile: number,
  labels: MetricLabels = {},
  scale = 1,
): TimeSeriesPoint[] {
  return history.slice(1).flatMap((frame, index) => {
    const value = histogramQuantile(
      history[index],
      frame,
      name,
      quantile,
      labels,
    );
    return value === null
      ? []
      : [{ timestamp: frame.timestamp, value: value * scale }];
  });
}

export function buildPrefixHitRateSeries(
  history: MetricsFrame[],
): TimeSeriesPoint[] {
  return history.slice(1).flatMap((frame, index) => {
    const before = history[index];
    const hitRate = counterRate(
      before,
      frame,
      "llm_prefix_cache_requests_total",
      { status: "hit" },
    );
    const missRate = counterRate(
      before,
      frame,
      "llm_prefix_cache_requests_total",
      { status: "miss" },
    );
    const total = hitRate + missRate;
    return total === 0
      ? []
      : [{ timestamp: frame.timestamp, value: (hitRate / total) * 100 }];
  });
}

export function buildAverageSeries(
  history: MetricsFrame[],
  name: string,
  labels: MetricLabels = {},
  scale = 1,
): TimeSeriesPoint[] {
  return history.slice(1).flatMap((frame, index) => {
    const before = history[index];
    const countRate = counterRate(before, frame, `${name}_count`, labels);
    if (countRate === 0) {
      return [];
    }
    const sumRate = counterRate(before, frame, `${name}_sum`, labels);
    return [
      {
        timestamp: frame.timestamp,
        value: (sumRate / countRate) * scale,
      },
    ];
  });
}

export function metricLabelValues(
  history: MetricsFrame[],
  name: string,
  label: string,
): string[] {
  const values = new Set<string>();
  for (const frame of history) {
    for (const sample of frame.samples) {
      const value = sample.labels[label];
      if (sample.name === name && value) {
        values.add(value);
      }
    }
  }
  return [...values].sort();
}
