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
  <strong>一个用于学习 batching、streaming、token scheduling 和 cache-aware inference system 的小型 LLM serving control plane。</strong>
</p>

<p align="center">
  <a href="./README.md">English</a>
  ·
  <a href="./docs">文档</a>
  ·
  <a href="#快速开始">快速开始</a>
</p>

---

## 项目概览

`mini-llm-serve` 是一个紧凑的 LLM serving system，重点放在模型执行外围的 **serving control plane**。

它不是 vLLM、TensorRT-LLM、SGLang 或 llama.cpp 的替代品。它的目标是把调度与系统层抽出来，让 LLM serving 的核心问题可以被端到端地学习、运行和观测：

- 请求生命周期管理
- prefill / decode separation
- token-budget-based scheduling
- streaming response delivery
- TTFT / TBT 可观测性
- prefix cache metadata
- executor dispatch 与结果回流
- 可复现 benchmark 场景

当前执行后端是 Python mock executor。这样做的目的是先让 scheduler 行为变得清晰、可测，再考虑真实 GPU 推理。

## 设计动机

现代 LLM serving stack 很强，但也很大，很难从第一性原理完整理解。

这个项目采取相反的方式：

- 足够小，方便阅读
- 足够真实，能暴露 serving tradeoff
- 结构足够清晰，后续可以继续扩展生产系统组件

设计目标不是 toy demo，而是一个最小可运行的 LLM inference serving control plane。

## 功能亮点

| 方向 | 当前能力 |
|---|---|
| API | Connect RPC inference service 与 admin/metrics endpoint |
| Control plane | Go 请求生命周期、scheduler、executor manager、metrics |
| Execution backend | 通过 Connect RPC 调用的 Python mock LLM executor |
| Scheduling | prefill/decode separation、token budget、small/large prefill queues |
| Streaming | unary 和 server-streaming generation path |
| Observability | Prometheus metrics、runtime stats、TTFT、TBT、queue wait、batch size |
| Cache model | prefix cache metadata、hit/miss、saved-token metrics |
| Benchmarks | cache miss、cache hit、mixed prompt workload |

## 架构

Mini LLM Serve 的调度机制是 token-aware work scheduling。请求由生命周期状态机管理，prefill 和 decode 则作为不同的 work item 进入调度器。

![Mini LLM Serve Architecture](./assets/Stage2_Architecture.svg)

最核心的内部循环是：

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

这个拆分让职责边界更清楚：

- `Request` 表示用户请求的完整生命周期。
- `WorkItem` 表示一次可调度执行单元。
- `Event` 表示 executor 输出，并驱动状态机继续推进。
- `Scheduler` 根据 sequence 和 token budget 选择 work。
- `ExecutorManager` 将 batch 分发给后端 executor。

## Benchmark

Benchmark 使用 Python mock executor，因此结果应该理解为 **control-plane behavior**，不是真实 GPU 推理性能。

![Mini LLM Serve Benchmark Summary](./assets/Stage2_Benchmark_Summary.svg)

工作负载：

- 每个场景 `1000` 请求
- 并发 `100`
- Go server + Python mock executor
- metrics 使用单次运行 delta

| Scenario | Throughput | Avg Latency | Avg TTFT | Avg TBT | Prefix Hits | Tokens Saved |
|---|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 3.28 req/s | 30.502s | 1.7322s | 0.4109s | 0 | 0 |
| `cache_hit` | 4.10 req/s | 24.341s | 0.3250s | 0.3430s | 1000 | 147000 |
| `mixed_prompt` | 4.22 req/s | 23.682s | 1.2117s | 0.3209s | 0 | 0 |

关键观察：

> 在当前 mock workload 下，prefix cache metadata 将平均 TTFT 从 `1.7322s` 降到 `0.3250s`，约 `81%` 降低。

更详细的报告和 benchmark 记录位于 [`docs`](./docs)。

## 快速开始

### 1. 启动 Python mock executor

```bash
cd llm_serve
make run
```

默认监听：`127.0.0.1:19991`

### 2. 启动 Go server

```bash
make run
```

默认端点：

- inference service: `127.0.0.1:8800`
- admin / metrics: `127.0.0.1:8801`

### 3. 查看 metrics

```bash
curl http://127.0.0.1:8801/metrics
```

### 4. 运行 benchmark

```bash
make bench-smoke
make bench-cache-miss
make bench-cache-hit
make bench-mixed-prompt
```

也可以直接通过 CLI 覆盖 benchmark 参数：

```bash
go run ./cmd/bench --mode mixed_prompt --requests 1000 --concurrency 50 --timeout-ms 15000
```

## 项目结构

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

## 文档

更详细的系统总结、benchmark 记录和实现计划都位于 [`docs`](./docs)。

## Non-Goals

这个仓库刻意不实现：

- 真实 GPU kernel
- 真实 KV block allocation
- PagedAttention 或 FlashAttention
- 分布式多节点推理
- 生产级 autoscaling
- 完整 OpenAI API 兼容

这些属于 inference engine 或生产平台范畴。本项目聚焦 serving control plane。

## 相关系统

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [Ollama](https://github.com/ollama/ollama)
- [Ray](https://github.com/ray-project/ray)
