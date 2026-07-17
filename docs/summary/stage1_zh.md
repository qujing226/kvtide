# Stage 1 总结

[English Version](./stage1_en.md)

## 概述

Stage 1 使 `kvtide` 具备了一条完整可运行的 LLM serving 主链路。系统通过基于 Connect 的网关接收推理请求，将请求放入 FIFO 队列，利用基于超时的动态批处理构建 batch，通过 Go 侧 worker 发起执行，并把批次转发到 Python mock executor。

Stage 1 的目标不是实现推理引擎本身，而是验证 serving pipeline、调度器行为、可观测性接口以及 benchmark 方法论。在这个阶段结束时，项目已经具备完整请求链路、运行时指标、管理接口、压测工具以及后续调度演进所需的基线实现。

## 关键结论

- Stage 1 已经建立了一条完整可运行的 serving pipeline，并实现了 FIFO 排队与基于超时的动态批处理。
- 与 no batching 相比，dynamic batching 显著提升了系统吞吐。
- batch size 同时受到到达速率与 batching timeout 的影响，因此形成了可观测的吞吐-延迟 tradeoff。
- FIFO 是一个合理的 Stage 1 基线方案，但它并不感知请求成本、prompt 长度或 token 级调度信息。

## 系统架构

![Stage 1 Architecture](../../assets/Stage1_Architecture.svg)

Stage 1 的架构可以分为三个平面。推理平面负责请求接入、队列管理、批处理构建、执行分发和结果回填。管理平面负责暴露 runtime stats 和 `/metrics`。执行后端则由独立的 Python mock executor 表示，这使得调度器与执行器之间的边界更加清晰。

这种分层很重要，因为它让 serving 控制流更容易解释。Go 侧负责请求生命周期管理、批处理、观测与错误处理，而 Python 侧只负责模拟模型执行并返回 batch 结果。

## 请求生命周期与批处理

![Stage 1 Request Lifecycle](../../assets/Stage1_Request_Lifecycle.svg)

请求生命周期可以拆成三个可观测阶段。`request_duration_seconds` 覆盖从请求到达到响应返回的完整端到端路径。`queue_wait_seconds` 描述请求入队后到被选入 batch 之前的等待时间。`execution_seconds` 则描述 worker 发起 executor RPC 之后的后端执行耗时。

这种生命周期划分的价值在于，它把调度引入的延迟和后端执行延迟分离开了，也使 timeout、cancel 和结果回填路径更容易解释。

![Stage 1 Batching and Scheduling](../../assets/Stage1_Batching%26Scheduling.svg)

批处理策略由两个 flush 条件驱动：batch size 和 batch timeout。当队列中请求数量达到预设 batch size 时，系统会立即 flush；如果最早进入队列的请求等待超过 batching timeout，系统也会触发 flush。

这两个条件分别解决不同问题。batch size 让系统在突发流量下充分利用批处理效率；batch timeout 则防止在稀疏流量下请求无限期等待。两者共同构成了 Stage 1 中最核心的吞吐-延迟 tradeoff。

## 压测结果

![Stage 1 Benchmark Sweep](../../assets/Stage1_Benchmark_Sweep.svg)

压测结果表明，动态批处理会以可观测、可量化的方式同时改变系统吞吐与延迟表现。

与 no batching 基线相比，dynamic batching 显著提升了吞吐能力。在当前工作负载下，`baseline_no_batching` 的吞吐约为 `10.26 req/s`，而 `dynamic_default` 提升到了 `67.22 req/s`，`dynamic_fastflush` 则进一步达到 `71.06 req/s`。

并发 sweep 展示了预期中的批处理 tradeoff。随着并发增加，系统会形成更大的 batch，并获得更高的吞吐；但与此同时，queue wait 与端到端延迟也会增加。在当前 mock workload 下，`dynamic_fastflush` 往往能比 `dynamic_default` 提供更好的平衡，因为它通过降低 queue wait 抵消了平均 batch size 略小的影响。

更详细的 benchmark 表格和原始结论可见 [Stage 1 Benchmark Notes](../benchmarks/stage1_zh.md)。

## 局限

FIFO 之所以适合作为 Stage 1 基线，是因为它简单、可观测、易于验证。但它也有明确局限：它无法区分请求的紧急程度与执行成本，也不会感知 prompt 长度和请求规模。在更真实的工作负载下，这容易产生 head-of-line blocking，使短请求被长请求或高成本请求拖慢。

因此，下一步不应只是继续微调 FIFO，而应进入更具 LLM 语义的调度阶段。最重要的两个方向是：基于 token budget 的调度，以及 prefill/decode 分离。这两项能力会让调度器更准确地感知请求成本，并为 streaming、cache-aware scheduling 以及更真实的 serving 行为打下基础。

