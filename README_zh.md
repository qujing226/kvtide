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
  <strong>一个 Go 实现的 LLM serving control plane，用于研究 token-aware scheduling、streaming observability、prefix cache 和 KV block-aware inference behavior。</strong>
</p>

<p align="center">
  <a href="./README.md">English</a>
  ·
  <a href="./docs">文档</a>
  ·
  <a href="#快速开始">快速开始</a>
  ·
  <a href="./docs/benchmarks/stage3_zh.md">Benchmark 报告</a>
</p>

---

## 项目定位

`mini-llm-serve` 聚焦 LLM serving 中的调度与控制平面。

它会把一个推理请求拆成由生命周期管理的 prefill/decode work，通过 token budget 做 mixed batching，流式返回生成结果，并分别观测 TTFT/TBT。同时，它用 mock tokenizer、prefix cache 和 KV block manager 模拟现代推理系统里的 cache-aware 行为。

它不是模型 runtime，也不是 vLLM、SGLang、TensorRT-LLM、llama.cpp 或 Ollama 的替代品。当前执行后端是 Python mock executor，目的是让 Go serving control plane 的行为可以被阅读、测试、压测和解释，而不是依赖 GPU 才能看清系统。

这个项目要回答的问题是：

- 一个请求如何经历 prefill、decode、streaming 和 cleanup？
- 为什么 LLM serving 里 `Request != WorkItem`？
- token budget 相比 request-level FIFO 改变了什么？
- TTFT 和 TBT 分别暴露什么瓶颈？
- prefix cache 到底节省了什么？
- KV block pressure 如何影响 batch efficiency 和 cache churn？

## 当前能力

| 方向 | 当前实现 |
|---|---|
| API | Connect RPC inference service、streaming generation、admin endpoints |
| Request lifecycle | Go 状态机管理 queued、prefill、decode、finished、timeout、canceled、failed |
| Scheduling | prefill/decode separation、token budget、small/large prefill queues、mixed batch |
| Execution | executor manager 将 batch 分发给 Python mock inference executor |
| Streaming | unary 与 server-streaming generation path，按 chunk 记录指标 |
| Tokenization | mock tokenizer 将 prompt 转成稳定 token ID |
| Prefix cache | user_id 作为 cache salt，完整 block hash 匹配，记录 hit/miss/saved tokens |
| KV block model | block table、free queue、cached blocks、eviction counter、allocation failure metrics |
| Observability | Prometheus 指标：queue wait、execution time、TTFT、TBT、batch size、work items、KV blocks |
| Benchmarks | quick regression profile 与 report profile，覆盖 cache miss、cache hit、mixed prompt、block pressure |

## 架构

Mini LLM Serve 的调度机制是 token-aware work scheduling。请求是生命周期对象，prefill 和 decode 是不同的可调度 work item。

![Mini LLM Serve Architecture](./assets/Stage2_Architecture.svg)

核心链路：

```text
GenerateRequest
  -> tokenizer
  -> Request lifecycle manager
  -> prefix cache lookup
  -> prefill/decode WorkItem
  -> token budget scheduler
  -> ExecutorManager
  -> Python mock executor
  -> Event
  -> next WorkItem or final response
  -> KV block cleanup
```

关键边界：

- `Request` 负责用户可见的完整生命周期和最终响应。
- `WorkItem` 是 scheduler 可以打包和分发的执行单元。
- `Event` 是 executor 的输出，用于驱动状态机继续推进。
- `Scheduler` 在 sequence budget 和 token budget 下选择 mixed prefill/decode work。
- `BlockManager` 模拟 prefix hit、block allocation、free-list reuse 和 eviction。
- `ExecutorManager` 将后端 executor 与 scheduler 解耦。

## Benchmark 摘要

Benchmark 使用 Python mock executor，因此结果应理解为 **serving control-plane behavior**，不是真实 GPU 推理性能。

![Mini LLM Serve Benchmark Summary](./assets/Stage3_Benchmark_Summary.svg)

工作负载：

- `cache_miss`：1000 请求，并发 100，唯一用户
- `cache_hit`：1000 请求，并发 100，10 个已 warmup 的 cache user
- `mixed_prompt`：1000 请求，并发 100，混合 short/medium/long prompt
- `block_pressure`：320 请求，并发 32，长 prompt 下的 KV block pressure

| Scenario | Throughput | Avg Latency | Avg TTFT | Avg TBT | Avg Batch | Prefix Hits | Tokens Saved | Evictions |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 4.40 req/s | 22.693s | 1.6175s | 0.3010s | 5.50 | 0 | 0 | 4490 |
| `cache_hit` | 4.90 req/s | 20.380s | 0.8390s | 0.2791s | 5.95 | 1000 | 80000 | 460 |
| `mixed_prompt` | 4.91 req/s | 20.363s | 1.3555s | 0.2715s | 6.17 | 0 | 0 | 1475 |
| `block_pressure` | 2.38 req/s | 13.428s | 2.1182s | 0.1615s | 3.38 | 0 | 0 | 6164 |

关键观察：

- Prefix cache hit 将平均 TTFT 从 `1.6175s` 降到 `0.8390s`，约 `48%`。
- Cache hit 将吞吐从 `4.40 req/s` 提升到 `4.90 req/s`，原因是 prefill pressure 下降。
- Block pressure 将平均 batch size 压低到 `3.38`，并将 eviction 提高到 `6164`，暴露出明显 KV churn。
- Queue wait 基本保持在 `5ms` 左右，端到端延迟主要来自请求阶段行为和反复 decode。

完整报告见：[`docs/benchmarks/stage3_zh.md`](./docs/benchmarks/stage3_zh.md)。

## 快速开始

### Docker Compose

```bash
docker compose up --build -d
```

该命令会启动一组一对一本地拓扑：

```text
Go control plane -> Python mock executor
```

可用端点：

- inference service：`http://127.0.0.1:8800`
- admin / metrics：`http://127.0.0.1:8801`

检查容器状态和 metrics：

```bash
docker compose ps
curl http://127.0.0.1:8801/metrics
```

查看服务日志或停止环境：

```bash
docker compose logs -f
docker compose down
```

### 从源码启动

启动 Python mock executor：

```bash
cd llm_serve
make run
```

在仓库根目录启动 Go server：

```bash
make run
```

### 运行 Benchmark

```bash
make bench-quick
make bench-report
```

`bench-quick` 用于快速行为回归检查，`bench-report` 用于生成完整报告数据。

## 项目结构

```text
cmd/
  bench/        benchmark CLI
  client/       simple client wrapper
  server/       Go serving process
internal/
  block/        KV block table、prefix matching、free queue、eviction model
  executor/     executor manager and Connect backend
  handler/      request admission and streaming output
  metrics/      Prometheus metrics and runtime stats
  model/        Request, WorkItem, Event, Batch, block metadata
  scheduler/    token-budget scheduler and prefill/decode queues
  state/        request lifecycle state machine
  tokenizer/    mock tokenizer
  transport/    Connect RPC transport handlers
llm_serve/      Python mock executor
proto/          protobuf API definitions
docs/           reports, plans, benchmark notes
k8s/            local Kubernetes manifests
```

## 范围边界

这个仓库刻意聚焦 serving control plane，不实现：

- CUDA kernel
- PagedAttention kernel
- FlashAttention kernel
- 真实 GPU KV tensor
- tensor parallel communication
- 生产级 autoscaling
- 完整 OpenAI API 兼容

这些属于 inference engine 或生产平台范畴。本项目建模的是推理执行外围的控制平面：生命周期、调度、streaming、cache metadata、KV block pressure 和可观测性。

## 相关系统

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [Ollama](https://github.com/ollama/ollama)
- [Ray](https://github.com/ray-project/ray)
