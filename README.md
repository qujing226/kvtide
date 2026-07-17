# KVTide

<p align="center">
  <img src="./assets/logo-horizontal.svg" alt="KVTide logo" width="420" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+" />
  <img src="https://img.shields.io/badge/Python-3.12%2B-3776AB?logo=python&logoColor=white" alt="Python 3.12+" />
  <img src="https://img.shields.io/badge/React-19-20232A?logo=react&logoColor=61DAFB" alt="React 19" />
  <img src="https://img.shields.io/badge/Connect-RPC-0B3954" alt="Connect RPC" />
  <img src="https://img.shields.io/badge/License-MIT-2E8B57" alt="MIT License" />
</p>

<p align="center">
  <strong>An interactive LLM serving control plane for studying request lifecycles, token-aware scheduling, prefix caching, KV block management, and streaming inference.</strong>
</p>

<p align="center">
  <a href="./README_zh.md">中文</a>
  ·
  <a href="#quick-start">Quick Start</a>
  ·
  <a href="#architecture">Architecture</a>
  ·
  <a href="./k8s/README.md">Kubernetes</a>
</p>

---

## Overview

`kvtide` isolates the control-plane mechanics behind modern LLM serving systems.

The Go server turns each inference request into schedulable prefill and decode work, builds mixed batches under sequence and token budgets, models prefix-cache and KV-block metadata, and streams generated chunks back to the client. A Python executor can run either the mock runner or the Qwen Transformers CPU runner.

This project is intended to make the following questions observable:

- How does a request move through queued, prefill, decode, streaming, and cleanup states?
- Why are `Request` and `WorkItem` different objects?
- How does token-aware batching differ from request-level FIFO?
- What do TTFT and TBT reveal about serving behavior?
- What does a prefix-cache hit save?
- How does KV block pressure affect scheduling and cache eviction?

KVTide includes a reference Qwen CPU executor, but it is not a replacement for optimized GPU runtimes such as vLLM, SGLang, or TensorRT-LLM.

## Website

The public website is also the runtime console:

- **Demo** sends a real Connect RPC stream through the Go control plane, renders Markdown output, and shows live Prometheus queue, batch, and KV-block metrics.
- **Lab** exposes the token-budget scheduler as a step-by-step experiment without changing production runtime state.
- **Blog** is an empty publishing surface for future design notes.

The browser only communicates with the Web service. Its same-origin proxy forwards the inference stream and the limited metrics endpoint to the internal Go service, so public deployments do not need to expose control-plane ports.

## Features

| Area | Implementation |
|---|---|
| API | Connect RPC unary and server-streaming inference endpoints |
| Request lifecycle | State machine for queued, prefill, decode, finished, timeout, canceled, and failed states |
| Scheduling | Separate prefill/decode queues, small/large prefill classification, mixed batches, sequence and token budgets |
| Execution | One Go control plane connected to one Python executor |
| Streaming | Incremental response chunks with TTFT, TBT, usage, and finish-reason tracking |
| Prefix cache | Per-user cache salt, full-block hash matching, hit/miss counters, and saved-token metrics |
| KV block model | Block tables, free-list reuse, cached blocks, allocation failures, and eviction counters |
| Observability | Prometheus metrics and runtime statistics for queues, batches, requests, latency, and KV blocks |
| Web interface | Project home, live runtime demo, scheduler lab, and design blog |
| Deployment | Docker Compose and local Kubernetes manifests for kind |

## Quick Start

Docker Compose is the recommended way to run the complete project:

```bash
cd executor
uv run hf download Qwen/Qwen3-0.6B --local-dir ./models/Qwen3-0.6B
cd ..
docker compose up --build -d
```

It starts a one-to-one topology:

```text
Browser
  -> Web container
  -> Go control plane
  -> Python Qwen executor
```

Open the web interface:

```text
http://127.0.0.1:5173
```

Available endpoints:

| Endpoint | Address |
|---|---|
| Web interface | `http://127.0.0.1:5173` |
| Inference service | `http://127.0.0.1:8800` |
| Admin and Prometheus metrics | `http://127.0.0.1:8801` |

Inspect or stop the stack:

```bash
docker compose ps
docker compose logs -f
curl http://127.0.0.1:8801/metrics
docker compose down
```

## Architecture

The project separates the user-visible request lifecycle from schedulable execution work:

- `Request` owns lifecycle state, streaming output, usage, and final completion.
- `WorkItem` represents one prefill or decode unit that can enter a batch.
- `Scheduler` selects mixed work under sequence and token budgets.
- `Event` carries executor results back into the lifecycle state machine.
- `BlockManager` models prefix matching, KV block allocation, reuse, and eviction.
- `ExecutorManager` dispatches each batch to the configured executor.

![KVTide architecture](./assets/Stage2_Architecture.svg)

The main request path is:

```text
GenerateRequest
  -> Tokenizer
  -> Request lifecycle manager
  -> Prefix-cache lookup and KV block allocation
  -> Prefill/decode WorkItem
  -> Token-budget scheduler
  -> Executor manager
  -> Python executor
  -> Event
  -> Next WorkItem or final streamed response
  -> KV block cleanup
```

The deployment intentionally uses one logical server and one logical executor. Adding replicas behind a Kubernetes Service would load-balance requests but would not create a coherent distributed KV-aware inference engine.

## Benchmark Profile

The benchmark uses the Python mock executor. The results describe control-plane behavior, not GPU inference performance.

![KVTide benchmark summary](./assets/Stage3_Benchmark_Summary.svg)

The bundled scenarios cover cache misses, warmed prefix-cache users, mixed prompt lengths, and KV block pressure. They are designed to expose changes in throughput, latency, TTFT, TBT, batch size, prefix hits, saved tokens, and eviction activity.

Historical benchmark details remain available in [`docs/benchmarks`](./docs/benchmarks).

Run the fast regression profile or the full report profile:

```bash
make bench-quick
make bench-report
```

## Run From Source

The recommended tool versions are declared in [`mise.toml`](./mise.toml). `kubectl` is intentionally not managed by mise because it commonly comes from Docker Desktop or the user's Kubernetes installation.

Install the web dependencies:

```bash
cd web
npm install
cd ..
```

Start the Python executor:

```bash
cd executor
make run
```

Start the Go server from the repository root in another terminal:

```bash
make run
```

Start the Vite development server in a third terminal:

```bash
make web-dev
```

Vite proxies inference traffic to `127.0.0.1:8800` and metrics traffic to `127.0.0.1:8801` during local development. The browser remains on the `5173` origin, so no development CORS configuration is required.


## Server Deployment

Use the production Compose overlay when deploying to a server without a domain:

```bash
KVTIDE_WEB_PORT=80 \
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up --build -d
```

Then open `http://<server-ip>/`. To use another public port, change only `KVTIDE_WEB_PORT`, for example `KVTIDE_WEB_PORT=8080`.

The production overlay publishes only the Web service. Ports `8800`, `8801`, and `19991` remain on the private Compose network. Open only the selected Web port in the host firewall or cloud security group.


## Kubernetes With kind

Build the images, create the local three-node cluster, and deploy the one-to-one server/executor topology:

```bash
make docker-build
make kube-start
make kube-forward
```

See [`k8s/README.md`](./k8s/README.md) for manifests, verification commands, probes, rollout behavior, and cleanup.

## Project Layout

```text
cmd/
  bench/        benchmark CLI
  client/       inference, executor, and admin clients
  server/       Go control-plane process
internal/
  block/        prefix matching, KV block tables, reuse, and eviction
  executor/     executor manager and Connect backend
  handler/      request admission and streaming output
  metrics/      Prometheus metrics and runtime statistics
  model/        Request, WorkItem, Event, Batch, and block metadata
  scheduler/    token-budget scheduler and prefill/decode queues
  state/        request lifecycle state machine
  tokenizer/    model-aware tokenizer registry
  transport/    Connect RPC, admin, and CORS handlers
executor/       Python executor runners
web/            React website, live runtime demo, scheduler lab, and blog shell
proto/          protobuf API definitions
k8s/            kind cluster configuration and Kubernetes manifests
docs/           historical summaries and benchmark reports
docker/         server, executor, and web images
```

## Scope

The repository focuses on the serving control plane. It does not implement:

- CUDA, PagedAttention, or FlashAttention kernels
- real GPU KV tensors
- tensor or pipeline parallelism
- a distributed KV cache
- production autoscaling
- full OpenAI API compatibility

These concerns belong to inference engines or production platforms. KVTide models the orchestration around execution: request lifecycle, scheduling, streaming, cache metadata, KV block pressure, and observability.

## Vision

KVTide is moving toward executor-aware KV mobility. Executors with the same model family, weights, dtype, and tensor-parallel configuration should be able to discover one another. When one executor's KV pressure becomes imbalanced, it should proactively push selected cache blocks to a compatible peer and update ownership without forcing the next request to recompute its prefix.

That direction requires executor-scoped block tables, runtime-epoch recovery, placement policy, transfer protocols, and measurements that distinguish useful reuse from transfer overhead. The current one-to-one runtime is the baseline for evaluating those mechanisms rather than pretending they already exist.

## Related Systems

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [Ollama](https://github.com/ollama/ollama)

## License

[MIT](./LICENSE)
