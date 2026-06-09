export interface BenchmarkScenario {
  id: "cache_miss" | "cache_hit" | "mixed_prompt" | "block_pressure";
  label: string;
  shortLabel: string;
  color: string;
  throughput: number;
  latency: number;
  ttft: number;
  tbt: number;
  batch: number;
  hits: number;
  savedTokens: number;
  evictions: number;
}

export const benchmarkScenarios: BenchmarkScenario[] = [
  {
    id: "cache_miss",
    label: "Cache miss",
    shortLabel: "MISS",
    color: "#dd6b2c",
    throughput: 4.4,
    latency: 22.693,
    ttft: 1.6175,
    tbt: 0.301,
    batch: 5.5,
    hits: 0,
    savedTokens: 0,
    evictions: 4490,
  },
  {
    id: "cache_hit",
    label: "Cache hit",
    shortLabel: "HIT",
    color: "#238363",
    throughput: 4.9,
    latency: 20.38,
    ttft: 0.839,
    tbt: 0.2791,
    batch: 5.95,
    hits: 1000,
    savedTokens: 80000,
    evictions: 460,
  },
  {
    id: "mixed_prompt",
    label: "Mixed prompt",
    shortLabel: "MIXED",
    color: "#367bb5",
    throughput: 4.91,
    latency: 20.363,
    ttft: 1.3555,
    tbt: 0.2715,
    batch: 6.17,
    hits: 0,
    savedTokens: 0,
    evictions: 1475,
  },
  {
    id: "block_pressure",
    label: "Block pressure",
    shortLabel: "PRESSURE",
    color: "#b83e36",
    throughput: 2.38,
    latency: 13.428,
    ttft: 2.1182,
    tbt: 0.1615,
    batch: 3.38,
    hits: 0,
    savedTokens: 0,
    evictions: 6164,
  },
];
