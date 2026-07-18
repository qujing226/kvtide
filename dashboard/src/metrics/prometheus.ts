export type MetricLabels = Record<string, string>;

export type MetricSample = {
  name: string;
  labels: MetricLabels;
  value: number;
};

export type MetricsFrame = {
  timestamp: number;
  samples: MetricSample[];
};

const samplePattern =
  /^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+([^\s]+)(?:\s+\d+)?$/;
const labelPattern = /([a-zA-Z_][a-zA-Z0-9_]*)="((?:\\.|[^"])*)"/g;

function parseLabels(rawLabels = ""): MetricLabels {
  const labels: MetricLabels = {};

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

export function parsePrometheusText(text: string): MetricSample[] {
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

    samples.push({ name, labels: parseLabels(rawLabels), value });
  }

  return samples;
}

function matchesLabels(sample: MetricSample, labels: MetricLabels): boolean {
  return Object.entries(labels).every(
    ([name, value]) => sample.labels[name] === value,
  );
}

export function sumMetric(
  samples: MetricSample[],
  name: string,
  labels: MetricLabels = {},
): number {
  return samples
    .filter(
      (sample) => sample.name === name && matchesLabels(sample, labels),
    )
    .reduce((total, sample) => total + sample.value, 0);
}

function counterDelta(before: number, after: number): number {
  return after >= before ? after - before : 0;
}

export function counterRate(
  before: MetricsFrame,
  after: MetricsFrame,
  name: string,
  labels: MetricLabels = {},
): number {
  const elapsedSeconds = (after.timestamp - before.timestamp) / 1000;
  if (elapsedSeconds <= 0) {
    return 0;
  }

  const beforeValue = sumMetric(before.samples, name, labels);
  const afterValue = sumMetric(after.samples, name, labels);
  return counterDelta(beforeValue, afterValue) / elapsedSeconds;
}

type HistogramBucket = {
  upperBound: number;
  cumulativeCount: number;
};

function histogramBuckets(
  before: MetricsFrame,
  after: MetricsFrame,
  name: string,
  labels: MetricLabels,
): HistogramBucket[] {
  const bucketName = `${name}_bucket`;
  const afterBuckets = after.samples.filter(
    (sample) =>
      sample.name === bucketName && matchesLabels(sample, labels) && sample.labels.le,
  );

  return afterBuckets
    .map((sample) => {
      const bucketLabels = { ...labels, le: sample.labels.le };
      const beforeValue = sumMetric(before.samples, bucketName, bucketLabels);
      return {
        upperBound:
          sample.labels.le === "+Inf" ? Number.POSITIVE_INFINITY : Number(sample.labels.le),
        cumulativeCount: counterDelta(beforeValue, sample.value),
      };
    })
    .filter((bucket) => !Number.isNaN(bucket.upperBound))
    .sort((left, right) => left.upperBound - right.upperBound);
}

export function histogramQuantile(
  before: MetricsFrame,
  after: MetricsFrame,
  name: string,
  quantile: number,
  labels: MetricLabels = {},
): number | null {
  const buckets = histogramBuckets(before, after, name, labels);
  const total = buckets.at(-1)?.cumulativeCount ?? 0;
  if (total <= 0 || quantile < 0 || quantile > 1) {
    return null;
  }

  const rank = quantile * total;
  let previousBound = 0;
  let previousCount = 0;

  for (const bucket of buckets) {
    if (bucket.cumulativeCount < rank) {
      previousBound = bucket.upperBound;
      previousCount = bucket.cumulativeCount;
      continue;
    }

    if (!Number.isFinite(bucket.upperBound)) {
      return previousBound;
    }

    const observations = bucket.cumulativeCount - previousCount;
    if (observations <= 0) {
      return bucket.upperBound;
    }

    const position = (rank - previousCount) / observations;
    return previousBound + (bucket.upperBound - previousBound) * position;
  }

  return null;
}
