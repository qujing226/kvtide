# Mini LLM Serve

<p align="center">
  <img src="./assets/logo-horizontal.svg" alt="Mini LLM Serve logo" width="420" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+" />
  <img src="https://img.shields.io/badge/Python-3.12%2B-3776AB?logo=python&logoColor=white" alt="Python 3.12+" />
  <img src="https://img.shields.io/badge/License-MIT-0B3954" alt="MIT License" />
  <img src="https://img.shields.io/badge/Docs-EN%20%7C%20ZH-E97132" alt="Docs EN ZH" />
  <img src="https://img.shields.io/badge/CI-passing-2E8B57" alt="CI passing" />
</p>

A compact LLM serving system for learning, experimentation, and scheduler prototyping.

[中文](./README_zh.md)

## What This Project Is

`mini-llm-serve` is a small but realistic LLM serving system focused on the serving control plane rather than the inference engine itself.

It is built to make the core serving problems easy to study end-to-end:

- request ingress via Connect RPC
- queueing and request lifecycle management
- dynamic batching
- prefill / decode separation
- token-budget-aware scheduling
- streaming response delivery
- pluggable executors
- Prometheus metrics and runtime stats
- reproducible benchmark scenarios

This repository is intentionally **not** a production-scale inference engine like vLLM, TensorRT-LLM, or llama.cpp. Instead, it focuses on the serving layer around model execution, with a clean enough structure to support scheduling, batching, streaming, and cache experiments.

## Why It Exists

Large LLM serving systems are powerful, but they are often too large to understand end-to-end. Many smaller demos, on the other hand, are too shallow to expose real systems tradeoffs.

The goal is straightforward: keep the system small enough to read, but real enough to expose serving tradeoffs:

- small enough to read in one sitting
- real enough to expose throughput / latency tradeoffs
- structured enough to extend with scheduler, cache, and streaming experiments

## Core Capabilities

- Go control plane with Connect RPC services
- separate inference and admin/metrics endpoints
- FIFO queueing baseline with dynamic batching
- prefill / decode separation for LLM-aware execution modeling
- token-budget-oriented scheduling experiments
- streaming response path
- Python mock executor backend over `connect-python`
- Prometheus metrics and AdminService runtime stats
- benchmark CLI for fixed scenarios and concurrency sweeps

## Architecture

![Stage 1 Architecture](./assets/Stage1_Architecture.svg)

The system has three logical planes:

- **Inference plane**: request admission, queueing, batching, execution dispatch, and result routing
- **Execution plane**: backend executor interface and Python mock executor
- **Observability plane**: admin service, runtime stats, and Prometheus metrics

This separation keeps the request flow easy to reason about while making the scheduler-executor boundary explicit.

## Quick Start

### 1. Start the Python mock executor

```bash
cd llm_serve
make run
```

By default, the Python executor listens on `127.0.0.1:19991`.

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

You can override benchmark parameters through the benchmark CLI:

```bash
go run ./cmd/bench --mode mixed_prompt --requests 1000 --concurrency 50 --timeout-ms 15000
```

## Stage 1 Benchmark Snapshot

![Stage 1 Benchmark Sweep](./assets/Stage1_Benchmark_Sweep.svg)

In the current benchmark setup:

- dynamic batching improves throughput substantially over the no-batching baseline
- larger batches improve throughput but increase queue wait and end-to-end latency
- backend execution remains stable, so the main differences come from scheduling policy
- flush policy changes the balance between queue wait and effective batch size

Detailed benchmark tables and observations are documented separately:

- [Stage 1 Benchmarks](./docs/benchmarks/stage1_en.md)
- [Stage 2 Benchmarks](./docs/benchmarks/stage2_en.md)

Stage 2 benchmark scenarios now focus on mixed prompt sizes, TTFT/TBT, and prefix cache hit/miss behavior.

## Documentation

Detailed reports and benchmark notes are available under `docs/`.

Stage reports:

- [Stage 1 Report](./docs/summary/stage1_en.md)

Benchmark notes:

- [Stage 1 Benchmarks](./docs/benchmarks/stage1_en.md)
- [Stage 2 Benchmarks](./docs/benchmarks/stage2_en.md)

Design and roadmap:

- [Stage 2 Plan](./docs/plans/2026-04-01-stage2-implementation-plan.md)
- [Project Extension Roadmap](./docs/plans/2026-03-27-project-extension-roadmap.md)

## Related Systems

- [vLLM](https://github.com/vllm-project/vllm)
- [Ollama](https://github.com/ollama/ollama)
- [Ray](https://github.com/ray-project/ray)
- [SGLang](https://github.com/sgl-project/sglang)
