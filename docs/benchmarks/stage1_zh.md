# Stage 1 Benchmark 记录

[English Version](./stage1_en.md)

本文记录 `kvtide` 在 Stage 1 阶段的 benchmark 结果。

Stage 1 范围：

- unary request path
- FIFO queue
- dynamic batching
- Go control plane + Python mock executor
- Prometheus metrics + AdminService runtime stats

## 测试设置

工作负载：

- client: `cmd/bench`
- backend executor: 通过 Connect RPC 调用的 Python mock executor
- mock execution latency: 单请求约 `138ms`
- target server: Go inference service，监听 `:8800`
- admin/metrics server: `:8801`

测试场景：

- `baseline_no_batching`
- `dynamic_default`
- `dynamic_fastflush`

主要指标：

- client 侧：throughput、avg/p50/p90/p99 latency
- server 侧：batches total、平均 batch size、平均 queue wait、平均 execution time

## 固定场景对比

这组结果直接比较了 Stage 1 的三种模式。

| Mode | Requests | Concurrency | Success | Throughput (req/s) | Avg Latency | P50 | P90 | P99 | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `baseline_no_batching` | 300 | 10 | 300 | 10.26 | 959ms | 1.000s | 1.002s | 1.005s | 1.00 | 0.8158 | 0.1380 |
| `dynamic_default` | 1000 | 100 | 1000 | 67.22 | 1.45s | 1.403s | 1.618s | 1.804s | 9.71 | 0.0637 | 0.1380 |
| `dynamic_fastflush` | 1000 | 100 | 1000 | 71.06 | 1.289s | 1.400s | 1.439s | 1.493s | 8.06 | 0.0100 | 0.1380 |

## Dynamic Default 并发 Sweep

| Concurrency | Requests | Success | Throughput (req/s) | Avg Latency | P50 | P90 | P99 | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 1000 | 1000 | 26.70 | 374ms | 291ms | 573ms | 853ms | 2.07 | 0.0042 | 0.1380 |
| 50 | 1000 | 1000 | 71.05 | 684ms | 573ms | 1.269s | 1.439s | 3.44 | 0.0049 | 0.1380 |
| 100 | 1000 | 1000 | 86.49 | 1.116s | 1.397s | 1.420s | 1.460s | 5.92 | 0.0076 | 0.1380 |
| 200 | 1000 | 1000 | 137.76 | 1.361s | 1.414s | 1.520s | 1.582s | 8.47 | 0.0231 | 0.1380 |

## Dynamic Fastflush 并发 Sweep

| Concurrency | Requests | Success | Throughput (req/s) | Avg Latency | P50 | P90 | P99 | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 1000 | 1000 | 18.60 | 535ms | 433ms | 990ms | 997ms | 2.35 | 0.0045 | 0.1380 |
| 50 | 1000 | 1000 | 61.74 | 798ms | 711ms | 1.407s | 1.445s | 4.18 | 0.0049 | 0.1380 |
| 100 | 1000 | 1000 | 99.71 | 958ms | 985ms | 1.427s | 1.518s | 4.44 | 0.0069 | 0.1380 |
| 200 | 1000 | 1000 | 139.30 | 1.213s | 1.410s | 1.498s | 1.535s | 6.58 | 0.0158 | 0.1380 |

## 关键结论

### 1. Dynamic batching 明显提升了吞吐

与 `baseline_no_batching` 相比，两种 dynamic batching 模式都带来了显著更高的吞吐。

- `baseline_no_batching`: `10.26 req/s`
- `dynamic_default`: `67.22 req/s`
- `dynamic_fastflush`: `71.06 req/s`

在当前 mock workload 下，dynamic batching 相比 no-batching baseline 大约提升了 `6.5x` 到 `7x` 的吞吐。

### 2. 后端执行稳定，主要差异来自调度策略

在所有场景中，平均 execution time 都稳定在约 `0.138s`。

这说明性能差异主要来自请求调度与 batching policy，而不是后端执行本身不稳定。

### 3. 更高并发会提升吞吐与批次质量

对于两种 dynamic 模式：

- throughput 会随着 concurrency 提升
- 平均 batch size 会随着 concurrency 提升
- queue wait 也会随着 concurrency 提升

这符合 batching serving system 的典型行为：更高并发更容易形成大 batch，但排队延迟也会作为代价一起上升。

### 4. `dynamic_default` 与 `dynamic_fastflush`

在当前 workload 下：

- 低并发时，`dynamic_default` 的表现优于 `dynamic_fastflush`
- 更高并发下，两者都具有良好的扩展性
- 在 `concurrency=200` 时，`dynamic_fastflush` 达到了略高的峰值吞吐
- `dynamic_default` 则更稳定地形成了更大的 batch

这意味着更优的 batching policy 会依赖请求到达模式：

- 较低并发更适合稍微等待，以形成更大的 batch
- 较高并发则更适合更快 flush，以降低等待开销

## Stage 1 总结

Stage 1 已经展示出一个小但真实的 LLM serving runtime 所应具备的核心行为：

- request queueing
- dynamic batching
- backend executor routing
- 端到端延迟埋点
- 可 benchmark 的性能 tradeoff