# Stage 2 Benchmarks

[中文版本](./stage2_zh.md)

This document records the Stage 2 benchmark results for KVTide.

Stage 2 scope:

- prefill / decode separation
- token-budget-aware scheduling
- streaming-oriented TTFT / TBT metrics
- prefix cache metadata
- benchmark metrics based on per-run deltas

The executor is still a Python mock backend. These numbers should be read as serving-control-plane behavior, not real GPU inference performance.

## Setup

Workload:

- client: `cmd/bench`
- backend executor: Python mock executor over Connect RPC
- target server: Go inference service on `:8800`
- admin/metrics server: `:8801`
- requests per scenario: `1000`
- concurrency per scenario: `100`
- timeout per request: `60s`

Scenarios:

- `cache_miss`: all requests use unique cache keys
- `cache_hit`: one warmup request builds the shared prefix cache entry, then all measured requests reuse it
- `mixed_prompt`: short, medium, and long prompts are mixed with unique cache keys

Primary metrics:

- client-side: throughput, avg/p50/p90/p99 latency
- server-side: batches total, average batch size, queue wait, execution time
- Stage 2 metrics: TTFT, TBT, prefix cache hits/misses, prefix tokens saved

## Scenario Results

| Mode | Requests | Concurrency | Success | Total | Throughput (req/s) | Avg Latency | P50 | P90 | P99 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 1000 | 100 | 1000 | 5m5.281s | 3.28 | 30.502s | 32.405s | 37.439s | 38.265s |
| `cache_hit` | 1000 | 100 | 1000 | 4m3.724s | 4.10 | 24.341s | 21.067s | 37.050s | 37.238s |
| `mixed_prompt` | 1000 | 100 | 1000 | 3m57.028s | 4.22 | 23.682s | 24.888s | 29.249s | 30.581s |

## Server Metrics

| Mode | Batches Total | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) | Queue Rejected | Observed Requests |
|---|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 14743 | 4.88 | 0.0055 | 0.0131 | 0 | 1000 |
| `cache_hit` | 12188 | 5.83 | 0.0053 | 0.0125 | 0 | 1000 |
| `mixed_prompt` | 11623 | 6.19 | 0.0047 | 0.0128 | 0 | 1000 |

## Stage 2 Metrics

| Mode | Avg TTFT (s) | Avg TBT (s) | Prefix Hits | Prefix Misses | Prefix Tokens Saved |
|---|---:|---:|---:|---:|---:|
| `cache_miss` | 1.7322 | 0.4109 | 0 | 1000 | 0 |
| `cache_hit` | 0.3250 | 0.3430 | 1000 | 0 | 147000 |
| `mixed_prompt` | 1.2117 | 0.3209 | 0 | 1000 | 0 |

## Key Findings

### 1. Prefix cache metadata reduces TTFT

Compared with `cache_miss`, `cache_hit` reduces average TTFT from `1.7322s` to `0.3250s`.

That is an approximate `81%` reduction.

This matches the expected behavior: when the prefix cache metadata hits, the request can skip most prefill work and reach decode earlier.

### 2. Cache hits improve throughput and average latency

Compared with `cache_miss`:

- throughput improves from `3.28 req/s` to `4.10 req/s`
- average latency drops from `30.502s` to `24.341s`
- total batch count drops from `14743` to `12188`
- average batch size rises from `4.88` to `5.83`

This indicates that reducing prefill pressure improves the scheduler's ability to keep decode work moving.

### 3. Queue wait is not the bottleneck in this workload

Average queue wait stays around `4.7ms` to `5.5ms` across all scenarios.

The main latency difference comes from request phase behavior:

- prefill cost affects TTFT
- repeated decode steps affect TBT and end-to-end latency
- cache hits reduce prefill pressure but do not eliminate decode-loop cost

### 4. Mixed prompt workload is faster than pure long-prompt cache miss

`mixed_prompt` has no prefix cache benefit, but it still outperforms `cache_miss`.

This is expected because the workload includes short and medium prompts, so average prefill cost is lower than the pure long-prompt `cache_miss` scenario.

### 5. Decode remains the next bottleneck

Even with prefix cache hits, average latency remains high under `1000` requests and `100` concurrency.

The reason is structural: the current Stage 2 model emits one decode token per execution step, which creates many decode work items and repeated scheduler/executor round trips.

This is intentionally left for a later experiment with multi-token decode chunks.

## Stage 2 Benchmark Summary

Stage 2 demonstrates the core behavior expected from a minimal LLM-aware serving runtime:

- prefill and decode are observable as different request phases
- TTFT and TBT expose different latency causes
- prefix cache metadata measurably reduces TTFT
- token-budget scheduling can run mixed prompt workloads without queue rejection
- benchmark metrics now use per-run deltas, making repeated comparisons safer
