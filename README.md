# Mini LLM Serve

<p align="center">
  <img src="./assets/logo-horizontal.svg" alt="Mini LLM Serve logo" width="420" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+" />
  <img src="https://img.shields.io/badge/Python-3.12%2B-3776AB?logo=python&logoColor=white" alt="Python 3.12+" />
  <img src="https://img.shields.io/badge/Connect-RPC-0B3954" alt="Connect RPC" />
  <img src="https://img.shields.io/badge/Prometheus-Metrics-E6522C?logo=prometheus&logoColor=white" alt="Prometheus metrics" />
  <img src="https://img.shields.io/badge/License-MIT-2E8B57" alt="MIT License" />
</p>

<p align="center">
  <strong>A small LLM serving control plane for learning batching, streaming, token scheduling, and cache-aware inference systems.</strong>
</p>

<p align="center">
  <a href="./README_zh.md">中文</a>
  ·
  <a href="./docs/summary/stage2_en.md">Stage 2 Report</a>
  ·
  <a href="./docs/benchmarks/stage2_en.md">Benchmarks</a>
  ·
  <a href="#quick-start">Quick Start</a>
</p>

---

## What Is This?

`mini-llm-serve` is a compact LLM serving system focused on the **serving control plane** around model execution.

It does not try to replace vLLM, TensorRT-LLM, SGLang, or llama.cpp. Instead, it isolates the scheduling and systems layer so the core serving problems are easier to study end-to-end:

- request lifecycle management
- prefill / decode separation
- token-budget-based scheduling
- streaming response delivery
- TTFT / TBT observability
- prefix cache metadata
- executor dispatch and result routing
- reproducible benchmark scenarios

The execution backend is currently a Python mock executor. The point is to make scheduler behavior visible and testable before introducing real GPU inference.

## Why It Exists

Modern LLM serving stacks are powerful, but they are also large and difficult to understand from first principles.

This project takes the opposite approach:

- small enough to read
- real enough to expose serving tradeoffs
- structured enough to extend toward production-style components

The design goal is not "toy demo". It is a minimal, runnable model of the control plane behind LLM inference serving.

## Feature Highlights

| Area | What exists today |
|---|---|
| API | Connect RPC inference service and admin/metrics endpoints |
| Control plane | Go request lifecycle, scheduler, executor manager, metrics |
| Execution backend | Python mock LLM executor over Connect RPC |
| Scheduling | prefill/decode separation, token budget, small/large prefill queues |
| Streaming | unary and server-streaming generation paths |
| Observability | Prometheus metrics, runtime stats, TTFT, TBT, queue wait, batch size |
| Cache model | prefix cache metadata with hit/miss and saved-token metrics |
| Benchmarks | cache miss, cache hit, mixed prompt workloads |

## Architecture

Stage 2 moves the system from request-level batching to token-aware work scheduling.

![Stage 2 Architecture](./assets/Stage2_Architecture.svg)

The important internal loop is:

```text
GenerateRequest
  -> Request
  -> WorkItem
  -> Scheduler
  -> ExecutorManager
  -> Python Mock Executor
  -> Event
  -> next WorkItem or final response
```

This split keeps responsibilities clear:

- `Request` owns the user-visible lifecycle.
- `WorkItem` is one schedulable unit of execution.
- `Event` drives the state machine after executor output.
- `Scheduler` chooses work by sequence and token budget.
- `ExecutorManager` dispatches batches to backend executors.

## Benchmark Highlights

The Stage 2 benchmark uses a Python mock executor, so the results should be read as **control-plane behavior**, not GPU inference performance.

![Stage 2 Benchmark Summary](./assets/Stage2_Benchmark_Summary.svg)

Workload:

- `1000` requests per scenario
- `100` concurrency
- Go server + Python mock executor
- metrics computed as per-run deltas

| Scenario | Throughput | Avg Latency | Avg TTFT | Avg TBT | Prefix Hits | Tokens Saved |
|---|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 3.28 req/s | 30.502s | 1.7322s | 0.4109s | 0 | 0 |
| `cache_hit` | 4.10 req/s | 24.341s | 0.3250s | 0.3430s | 1000 | 147000 |
| `mixed_prompt` | 4.22 req/s | 23.682s | 1.2117s | 0.3209s | 0 | 0 |

Key observation:

> Prefix cache metadata reduced average TTFT from `1.7322s` to `0.3250s`, about an `81%` reduction in this mock workload.

Read the full benchmark notes:

- [Stage 2 Benchmarks](./docs/benchmarks/stage2_en.md)
- [Stage 1 Benchmarks](./docs/benchmarks/stage1_en.md)

## Quick Start

### 1. Start the Python mock executor

```bash
cd llm_serve
make run
```

The executor listens on `127.0.0.1:19991` by default.

### 2. Start the Go server

```bash
make run
```

Default endpoints:

- inference service: `127.0.0.1:8800`
- admin / metrics: `127.0.0.1:8801`

### 3. Check metrics

```bash
curl http://127.0.0.1:8801/metrics
```

### 4. Run benchmarks

```bash
make bench-smoke
make bench-cache-miss
make bench-cache-hit
make bench-mixed-prompt
```

Override benchmark parameters directly through the CLI:

```bash
go run ./cmd/bench --mode mixed_prompt --requests 1000 --concurrency 50 --timeout-ms 15000
```

## Project Layout

```text
cmd/
  bench/        benchmark CLI
  client/       simple client wrapper
  server/       Go serving process
internal/
  cache/        prefix cache metadata
  executor/     executor manager and Connect backend
  handler/      request admission
  metrics/      Prometheus metrics and runtime stats
  model/        Request, WorkItem, Event, Batch
  scheduler/    token-budget scheduler and queues
  state/        request lifecycle state machine
  transport/    Connect RPC transport handlers
llm_serve/      Python mock executor
proto/          protobuf API definitions
docs/           reports, plans, benchmark notes
```

## Documentation

Stage reports:

- [Stage 2 Report](./docs/summary/stage2_en.md)
- [Stage 1 Report](./docs/summary/stage1_en.md)

Benchmark notes:

- [Stage 2 Benchmarks](./docs/benchmarks/stage2_en.md)
- [Stage 1 Benchmarks](./docs/benchmarks/stage1_en.md)

Design and roadmap:

- [Stage 2 Plan](./docs/plans/2026-04-01-stage2-implementation-plan.md)
- [Project Extension Roadmap](./docs/plans/2026-03-27-project-extension-roadmap.md)

## Roadmap

- Multi-token decode chunks to reduce scheduler/RPC overhead
- Kubernetes deployment with router, service discovery, and metrics
- Real vLLM executor adapter
- Phase-specific batch metrics for prefill and decode
- More realistic load generation and request distributions

## Non-Goals

This repository intentionally does not implement:

- real GPU kernels
- real KV block allocation
- PagedAttention or FlashAttention
- distributed multi-node inference
- production autoscaling
- full OpenAI API compatibility

Those are inference-engine or production-platform concerns. This project focuses on the serving control plane.

## Related Systems

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [Ollama](https://github.com/ollama/ollama)
- [Ray](https://github.com/ray-project/ray)
