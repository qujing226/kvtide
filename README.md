# KVTide

![1](./assets/banner.svg)
<p align="center">
  <strong>KV-aware LLM serving, built from the runtime up.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+" />
  <img src="https://img.shields.io/badge/Python-3.12%2B-3776AB?logo=python&logoColor=white" alt="Python 3.12+" />
  <img src="https://img.shields.io/badge/React-19-20232A?logo=react&logoColor=61DAFB" alt="React 19" />
  <img src="https://img.shields.io/badge/Connect-RPC-0B3954" alt="Connect RPC" />
  <img src="https://img.shields.io/badge/License-MIT-2E8B57" alt="MIT License" />
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

## What is KVTide?

KVTide is an open LLM serving runtime that makes scheduling, request state, and KV-cache ownership explicit. A Go control plane tokenizes requests, schedules prefill and decode work under token budgets, manages prefix-cache and block metadata, and streams results. A Python executor runs either a synthetic mock workload or a real Qwen Transformers CPU forward path.

The current runtime intentionally uses **one control plane, one executor, and one KV block pool**. It is a coherent baseline for studying runtime behavior, not a claim that ordinary service replication creates distributed inference.

> **Vision:** KV cache should move toward available compute automatically. KVTide is working toward a Kubernetes-native runtime where compatible executors can proactively push reusable KV blocks to peers and update ownership without forcing prefix recomputation.

## Runtime Console

The website is part of the runtime rather than a static project page.

- **Demo / Topology** discovers the connected executor through `GetExecutors` and shows its model, runtime epoch, device, dtype, and KV capacity.
- **Demo / Request** sends a real Connect RPC stream through the Go control plane and renders the generated Markdown response.
- **Demo / Metrics** reads Prometheus metrics and explains queue, batch, request, latency, prefix-cache, and block-pool behavior.
- **Lab** provides a step-driven token-budget scheduling experiment isolated from production runtime state.

<p align="center">
  <img src="./assets/front-topology.png" alt="KVTide runtime topology" width="920" />
</p>

## Current Capabilities

| Area | Implementation |
|---|---|
| API | Connect RPC server-streaming inference plus executor and admin RPCs |
| Lifecycle | Queued, prefill, decode, streaming, finished, timeout, and failure transitions |
| Scheduling | Separate prefill/decode queues, chunked prefill, mixed work batches, sequence and token budgets |
| Prefix cache | User-scoped cache salt, chained full-block hashes, hit/miss and saved-token metrics |
| KV blocks | Logical block tables, free-list allocation, cached block reuse, rollback, release, and eviction accounting |
| Execution | Mock runner and Qwen3-0.6B Transformers CPU runner |
| KV tensors | Executor-side paged KV storage with a Hugging Face `DynamicCache` adapter |
| Streaming | Incremental text, TTFT/TBT observations, usage, finish reason, and errors |
| Observability | Prometheus metrics plus executor runtime inventory |
| Deployment | Docker Compose, a low-resource cloud Mock profile, and local Kubernetes manifests for kind |

## Architecture

KVTide separates the user-visible request from the work selected for one model execution step:

- `Request` owns input tokens, lifecycle state, generated output, usage, and completion.
- `WorkItem` describes one schedulable prefill chunk or decode step.
- `Scheduler` selects work under sequence and token budgets.
- `BlockManager` owns prefix matching and logical KV-block metadata.
- `ExecutorManager` sends a batch to the configured runtime.
- `Event` commits or rolls back block state and advances the request lifecycle.

![KVTide architecture](./assets/Stage2_Architecture.svg)

```text
GenerateStream
  -> model-aware tokenizer
  -> request state manager
  -> prefix lookup and block allocation
  -> prefill/decode WorkItem
  -> token-budget scheduler
  -> executor manager
  -> Python model runner
  -> Event
  -> next WorkItem or streamed completion
  -> block release / cache retention
```

The control plane sends token IDs, block tables, newly allocated block IDs, computed-token offsets, and a runtime epoch to the executor. The executor uses that metadata to reconstruct historical KV, write new slots, and reject work addressed to a stale runtime instance.

## Quick Start

### Real Qwen CPU Executor

The default Compose stack runs Qwen3-0.6B on CPU. Download the model first:

```bash
cd executor
uv run hf download Qwen/Qwen3-0.6B --local-dir ./models/Qwen3-0.6B
cd ..

docker compose up --build -d
```

Open `http://127.0.0.1:5173`.

| Service | Address |
|---|---|
| Web runtime console | `http://127.0.0.1:5173` |
| Inference API | `http://127.0.0.1:8800` |
| Admin API and metrics | `http://127.0.0.1:8801` |

```bash
docker compose ps
docker compose logs -f
curl http://127.0.0.1:8801/metrics
docker compose down
```

The default executor uses fp32 weights and a 512 MiB KV-cache budget. Use a machine with at least 8 GiB RAM for this profile.

## Benchmark

The benchmark uses the Mock Executor and measures control-plane behavior, not GPU kernel performance.

![KVTide benchmark summary](./assets/Stage3_Benchmark_Summary.svg)

The bundled profiles cover cold prefixes, warmed user-scoped prefixes, mixed prompt lengths, token-budget batching, and KV-block pressure. Reported signals include throughput, average and tail latency, TTFT, TBT, batch size, prefix hits, saved tokens, allocation failures, and evictions.

```bash
make bench-quick
make bench-report
```

Historical reports are retained in [`docs/benchmarks`](./docs/benchmarks).

## Kubernetes with kind

The Kubernetes manifests preserve the same one-to-one runtime topology:

```bash
make docker-build
make kube-start
make kube-forward
```

See [`k8s/README.md`](./k8s/README.md) for manifests, probes, rollout behavior, inspection commands, and cleanup.

The executor Deployment intentionally has one replica. Adding replicas behind a Kubernetes Service would load-balance batches without preserving executor-local KV ownership.

## Scope and Boundaries

Implemented today:

- A working Go scheduling and request-lifecycle control plane
- Real streaming RPC and Prometheus observability
- Model-aware Go tokenization
- A real CPU forward path with executor-side paged KV storage
- Prefix-cache and KV-block ownership metadata in a one-executor runtime

Not implemented yet:

- CUDA, FlashAttention, or a custom PagedAttention kernel
- Tensor, pipeline, or expert parallelism
- Mixed-phase execution in one GPU forward
- Multi-executor block namespaces and placement policy
- Peer-to-peer KV transfer
- A production Kubernetes Operator or autoscaler
- Full OpenAI API compatibility

## Roadmap: From Ownership to Mobility

The next architectural boundary is executor-aware block ownership. Each block table must be scoped by executor and runtime epoch before the control plane can recover safely from restarts or place work across replicas.

From there, KVTide can evaluate its central hypothesis: when compatible executors run the same model weights, dtype, and tensor-parallel configuration, an overloaded executor should be able to **push** selected KV blocks to an available peer. The control plane should observe the new placement, update metadata, and measure whether reuse saved more work than transfer consumed.

That path requires evidence, not only functionality. Future evaluations should compare recomputation, local reuse, and remote transfer across TTFT, TBT, throughput, transfer bandwidth, cache pressure, and tail latency.

## Related Systems

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [LMCache](https://github.com/LMCache/LMCache)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)

## License

[MIT](./LICENSE)
