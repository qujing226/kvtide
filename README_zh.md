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

一个面向学习、实验与调度原型设计的紧凑型 LLM serving system。

[English Version](./README.md)

## 这个项目是什么

`mini-llm-serve` 是一个小而真实的 LLM serving system，重点放在 serving control plane，而不是底层 inference engine 本身。

它希望把 serving 中最核心、也最值得学习的部分清晰地暴露出来：

- 基于 Connect RPC 的请求入口
- 请求排队与生命周期管理
- dynamic batching
- prefill / decode separation
- token-budget-aware scheduling
- streaming response delivery
- 可插拔 executors
- Prometheus 指标与 runtime stats
- 可复现的 benchmark 场景

这个仓库刻意 **不是** vLLM、TensorRT-LLM、llama.cpp 这类生产级推理引擎的替代品。它更关注模型执行外围的 serving 层，并且保持足够清晰的结构，以便继续做调度、批处理、streaming 和 cache 实验。

## 为什么要做它

大型 LLM serving 系统很强大，但通常太大，难以完整读懂；很多小 demo 虽然容易跑通，却不足以暴露真实系统中的 tradeoff。

目标很直接：系统要足够小，便于读懂；也要足够真实，能暴露 serving 中的关键 tradeoff：

- 足够小，能在较短时间内读懂
- 足够真，能看到吞吐与延迟的真实权衡
- 足够干净，后续能继续扩展 scheduler、cache 和 streaming 实验

## 核心能力

- Go 控制平面与 Connect RPC 服务
- inference 与 admin/metrics 端口分离
- 以 FIFO 为基线的排队与动态批处理
- 用于建模 LLM 执行差异的 prefill / decode separation
- 面向 token budget 的调度实验
- streaming 响应路径
- 基于 `connect-python` 的 Python mock executor
- Prometheus 指标与 AdminService runtime stats
- 支持固定场景与并发 sweep 的 benchmark CLI

## 架构概览

![Stage 1 Architecture](./assets/Stage1_Architecture.svg)

这个系统可以分成三个逻辑平面：

- **推理平面**：请求接入、排队、批处理、执行分发与结果回填
- **执行平面**：后端 executor 接口与 Python mock executor
- **观测平面**：admin service、runtime stats 与 Prometheus metrics

这种分层让请求流更容易理解，也使 scheduler 与 executor 之间的边界更明确。

## 快速开始

### 1. 启动 Python mock executor

```bash
cd llm_serve
make run
```

默认监听：`127.0.0.1:19991`

### 2. 启动 Go 服务

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

也可以直接通过 benchmark CLI 覆盖参数：

```bash
go run ./cmd/bench --mode mixed_prompt --requests 1000 --concurrency 50 --timeout-ms 15000
```

## Stage 1 Benchmark 摘要

![Stage 1 Benchmark Sweep](./assets/Stage1_Benchmark_Sweep.svg)

在当前 benchmark 配置下：

- dynamic batching 相比 no-batching baseline 显著提升吞吐
- 更大的 batch 往往提升吞吐，但也会增加 queue wait 和端到端延迟
- backend execution 相对稳定，因此主要差异来自调度策略
- flush policy 会改变 queue wait 与有效 batch size 之间的平衡

更详细的 benchmark 表格和原始结论见：

- [Stage 1 Benchmarks](./docs/benchmarks/stage1_zh.md)
- [Stage 2 Benchmarks](./docs/benchmarks/stage2_zh.md)

Stage 2 benchmark 场景现在重点覆盖 mixed prompt、TTFT/TBT，以及 prefix cache hit/miss 行为。

## 文档导航

更详细的报告和 benchmark 说明都放在 `docs/` 目录下。

阶段报告：

- [Stage 1 Report](./docs/summary/stage1_zh.md)

Benchmark 文档：

- [Stage 1 Benchmarks](./docs/benchmarks/stage1_zh.md)
- [Stage 2 Benchmarks](./docs/benchmarks/stage2_zh.md)

设计与路线图：

- [Stage 2 Plan](./docs/plans/2026-04-01-stage2-implementation-plan.md)
- [Project Extension Roadmap](./docs/plans/2026-03-27-project-extension-roadmap.md)

## 相关项目

- [vLLM](https://github.com/vllm-project/vllm)
- [Ollama](https://github.com/ollama/ollama)
- [Ray](https://github.com/ray-project/ray)
- [SGLang](https://github.com/sgl-project/sglang)
