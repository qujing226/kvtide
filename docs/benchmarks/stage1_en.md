# Stage 1 Benchmarks

[中文版本](./stage1_zh.md)

This document records the Stage 1 benchmark results for KVTide.

Stage 1 scope:

- unary request path
- FIFO queue
- dynamic batching
- Go control plane + Python mock executor
- Prometheus metrics + AdminService runtime stats

## Setup

Workload:

- client: `cmd/bench`
- backend executor: Python mock executor over Connect RPC
- mock execution latency: about `138ms` per request
- target server: Go inference service on `:8800`
- admin/metrics server: `:8801`

Scenarios:

- `baseline_no_batching`
- `dynamic_default`
- `dynamic_fastflush`

Primary metrics:

- client-side: throughput, avg/p50/p90/p99 latency
- server-side: batches total, average batch size, average queue wait, average execution time

## Fixed Scenario Comparison

These runs compare the three Stage 1 modes directly.

| Mode | Requests | Concurrency | Success | Throughput (req/s) | Avg Latency | P50 | P90 | P99 | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `baseline_no_batching` | 300 | 10 | 300 | 10.26 | 959ms | 1.000s | 1.002s | 1.005s | 1.00 | 0.8158 | 0.1380 |
| `dynamic_default` | 1000 | 100 | 1000 | 67.22 | 1.45s | 1.403s | 1.618s | 1.804s | 9.71 | 0.0637 | 0.1380 |
| `dynamic_fastflush` | 1000 | 100 | 1000 | 71.06 | 1.289s | 1.400s | 1.439s | 1.493s | 8.06 | 0.0100 | 0.1380 |

## Dynamic Default Concurrency Sweep

| Concurrency | Requests | Success | Throughput (req/s) | Avg Latency | P50 | P90 | P99 | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 1000 | 1000 | 26.70 | 374ms | 291ms | 573ms | 853ms | 2.07 | 0.0042 | 0.1380 |
| 50 | 1000 | 1000 | 71.05 | 684ms | 573ms | 1.269s | 1.439s | 3.44 | 0.0049 | 0.1380 |
| 100 | 1000 | 1000 | 86.49 | 1.116s | 1.397s | 1.420s | 1.460s | 5.92 | 0.0076 | 0.1380 |
| 200 | 1000 | 1000 | 137.76 | 1.361s | 1.414s | 1.520s | 1.582s | 8.47 | 0.0231 | 0.1380 |

## Dynamic Fastflush Concurrency Sweep

| Concurrency | Requests | Success | Throughput (req/s) | Avg Latency | P50 | P90 | P99 | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 1000 | 1000 | 18.60 | 535ms | 433ms | 990ms | 997ms | 2.35 | 0.0045 | 0.1380 |
| 50 | 1000 | 1000 | 61.74 | 798ms | 711ms | 1.407s | 1.445s | 4.18 | 0.0049 | 0.1380 |
| 100 | 1000 | 1000 | 99.71 | 958ms | 985ms | 1.427s | 1.518s | 4.44 | 0.0069 | 0.1380 |
| 200 | 1000 | 1000 | 139.30 | 1.213s | 1.410s | 1.498s | 1.535s | 6.58 | 0.0158 | 0.1380 |

## Key Findings

### 1. Dynamic batching clearly improves throughput

Compared with `baseline_no_batching`, both dynamic batching modes deliver much higher throughput.

- `baseline_no_batching`: `10.26 req/s`
- `dynamic_default`: `67.22 req/s`
- `dynamic_fastflush`: `71.06 req/s`

Under the current mock workload, dynamic batching improves throughput by roughly `6.5x` to `7x` over the no-batching baseline.

### 2. Backend execution is stable; scheduling is the main differentiator

Across all scenarios, average execution time remains close to `0.138s`.

This indicates that performance differences are primarily caused by request scheduling and batching policy rather than backend instability.

### 3. Higher concurrency increases throughput and batch quality

For both dynamic modes:

- throughput rises as concurrency increases
- average batch size rises with concurrency
- queue wait also rises with concurrency

This is expected behavior for a batching-based serving system: more concurrent requests make it easier to form larger batches, but queueing delay grows as a tradeoff.

### 4. `dynamic_default` vs `dynamic_fastflush`

Under this workload:

- at low concurrency, `dynamic_default` performs better than `dynamic_fastflush`
- at higher concurrency, both scale well
- `dynamic_fastflush` reaches slightly higher top-end throughput at `concurrency=200`
- `dynamic_default` forms larger batches more consistently

This suggests that the better batching policy depends on request arrival rate:

- lower concurrency benefits from waiting a bit longer to form batches
- higher concurrency benefits from faster flush and lower waiting overhead

## Stage 1 Summary

Stage 1 demonstrates that the system already has the core serving behaviors expected from a minimal LLM serving runtime:

- request queueing
- dynamic batching
- backend executor routing
- end-to-end latency instrumentation
- benchmarkable performance tradeoffs
