# Stage 3 Benchmarks

[中文版本](./stage3_zh.md)

This document records the Stage 3 benchmark results for KVTide.

Stage 3 extends the serving-control-plane model with:

- mock tokenizer and token IDs
- KV block manager
- prefix cache based on full-block hashes
- free-list block reuse and cache eviction metrics
- benchmark profiles for quick regression and report generation

The executor is still a Python mock backend. These numbers should be read as control-plane behavior, not real GPU inference performance.

![Stage 3 Benchmark Summary](../../assets/Stage3_Benchmark_Summary.svg)

## Setup

Runtime:

- client: `cmd/bench`
- server: Go inference service on `:8800`
- metrics: Prometheus endpoint on `:8801`
- backend: Python mock executor over Connect RPC
- batch policy: token-budget mixed prefill/decode scheduling
- KV model: temporary `16` tokens per block and `1024` total blocks

Benchmark profiles:

- `bench-quick`: small deterministic regression profile
- `bench-report`: full report profile used by this document

Scenarios:

- `cache_miss`: 1000 requests, concurrency 100, unique users
- `cache_hit`: 1000 requests, concurrency 100, 10 warmed cache users
- `mixed_prompt`: 1000 requests, concurrency 100, mixed short/medium/long prompts
- `block_pressure`: 320 requests, concurrency 32, long prompts that create KV block pressure

## Scenario Results

| Scenario | Requests | Concurrency | Success | Total | Throughput | Avg Latency | P50 | P90 | P99 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 1000 | 100 | 1000 | 3m47.141s | 4.40 req/s | 22.693s | 23.827s | 30.135s | 30.171s |
| `cache_hit` | 1000 | 100 | 1000 | 3m23.995s | 4.90 req/s | 20.380s | 19.873s | 25.492s | 27.818s |
| `mixed_prompt` | 1000 | 100 | 1000 | 3m23.775s | 4.91 req/s | 20.363s | 21.992s | 24.682s | 24.706s |
| `block_pressure` | 320 | 32 | 320 | 2m14.367s | 2.38 req/s | 13.428s | 13.003s | 15.403s | 16.538s |

## Scheduler Metrics

| Scenario | Batches | Avg Batch Size | Avg Queue Wait | Avg Execution | Queue Rejected | Observed Requests |
|---|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 13100 | 5.50 | 0.0055s | 0.0109s | 0 | 1000 |
| `cache_hit` | 12101 | 5.95 | 0.0054s | 0.0103s | 0 | 1000 |
| `mixed_prompt` | 11663 | 6.17 | 0.0053s | 0.0105s | 0 | 1000 |
| `block_pressure` | 6821 | 3.38 | 0.0055s | 0.0137s | 0 | 320 |

## Runtime Metrics

| Scenario | Avg TTFT | Avg TBT | Prefix Hits | Prefix Misses | Tokens Saved | Prefill Items | Decode Items | Cached Blocks | Allocation Failures | Evictions |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 1.6175s | 0.3010s | 0 | 1000 | 0 | 1000 | 71000 | 510 | 0 | 4490 |
| `cache_hit` | 0.8390s | 0.2791s | 1000 | 0 | 80000 | 1000 | 71000 | 50 | 0 | 460 |
| `mixed_prompt` | 1.3555s | 0.2715s | 0 | 1000 | 0 | 1000 | 71000 | 240 | 0 | 1475 |
| `block_pressure` | 2.1182s | 0.1615s | 0 | 320 | 0 | 320 | 22720 | 796 | 0 | 6164 |

At the end of every scenario:

- active requests returned to `0`
- inflight batches returned to `0`
- active KV blocks returned to `0`
- free KV blocks returned to `1024`

`cached` blocks are not exclusive with `free` blocks. A cached block can be idle in the free list and still be reusable by a later prefix match.

## Interpretation

### Prefix cache reduces prefill pressure

`cache_hit` reduced average TTFT from `1.6175s` to `0.8390s`, about a `48%` reduction compared with `cache_miss`.

The benchmark prompt contains more tokens than the full cached blocks cover, so every measured request still has one prefill work item for the uncached tail. The cache hit therefore does not eliminate prefill entirely; it reduces the prefill cost.

This is why `cache_hit` still reports:

- `prefill_items = 1000`
- `prefix_hits = 1000`
- `tokens_saved = 80000`

The important signal is not "zero prefill work". The important signal is that the cached prefix moves each request closer to decode and reduces TTFT.

### Cache hits improve batch efficiency

Compared with `cache_miss`, `cache_hit` reduced the number of batches from `13100` to `12101` and increased average batch size from `5.50` to `5.95`.

This means lower prefill pressure allows the scheduler to form slightly denser mixed batches and keep decode work moving with less churn.

### Mixed prompts expose workload composition effects

`mixed_prompt` has no prefix cache benefit, but it still reaches `4.91 req/s`, close to the cache-hit scenario.

The reason is workload composition: short and medium prompts reduce average prefill cost. This produces a better average batch size (`6.17`) than the all-long cache-miss workload (`5.50`).

This scenario is useful because it shows why request count alone is a weak scheduling signal. Token cost matters more than request count.

### Block pressure exposes KV churn

`block_pressure` uses fewer requests and lower concurrency, but much longer prompts. It reduces average batch size to `3.38` and increases evictions to `6164`.

This is the intended signal: KV block pressure lowers batching efficiency and creates more cache churn. The scenario completed without queue rejection or allocation failure, so the run measures pressure rather than an overloaded failure mode.

### Queue wait is not the primary bottleneck

Average queue wait stayed around `5ms` in all scenarios.

The large end-to-end latency values come from request lifecycle behavior:

- prefill affects TTFT
- decode loops affect TBT and total latency
- cache hits reduce prompt work but do not remove decode cost
- block pressure reduces batch density and increases eviction churn

## Takeaways

- TTFT and TBT need separate metrics because prefill pressure and decode-loop behavior have different causes.
- Prefix cache should be interpreted through saved tokens and TTFT, not just hit count.
- Token-aware scheduling is necessary because request count does not describe execution cost.
- KV block pressure is visible through average batch size, cached/free block state, evictions, and allocation failures.
- A mock executor can still expose control-plane behavior when metrics are tied to request lifecycle and scheduler decisions.
