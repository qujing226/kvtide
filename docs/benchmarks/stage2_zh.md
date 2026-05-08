# Stage 2 Benchmark 记录

[English Version](./stage2_en.md)

本文记录 Mini LLM Serve 在 Stage 2 阶段的 benchmark 结果。

Stage 2 范围：

- prefill / decode separation
- token-budget-aware scheduling
- 面向 streaming 的 TTFT / TBT 指标
- prefix cache metadata
- 基于单次运行 delta 的 benchmark metrics

当前 executor 仍然是 Python mock backend。因此这些数字应该理解为 serving control plane 的系统行为，而不是真实 GPU 推理性能。

## 测试设置

工作负载：

- client: `cmd/bench`
- backend executor: 通过 Connect RPC 调用的 Python mock executor
- target server: Go inference service，监听 `:8800`
- admin/metrics server: `:8801`
- 每个场景请求数：`1000`
- 每个场景并发数：`100`
- 单请求超时：`60s`

测试场景：

- `cache_miss`：所有请求使用唯一 cache key
- `cache_hit`：先执行 1 个 warmup 请求建立 shared prefix cache，再正式压测复用同一个 cache key
- `mixed_prompt`：混合 short / medium / long prompt，并使用唯一 cache key

主要指标：

- client 侧：throughput、avg/p50/p90/p99 latency
- server 侧：batches total、平均 batch size、平均 queue wait、平均 execution time
- Stage 2 指标：TTFT、TBT、prefix cache hits/misses、prefix tokens saved

## 场景结果

| Mode | Requests | Concurrency | Success | Total | Throughput (req/s) | Avg Latency | P50 | P90 | P99 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 1000 | 100 | 1000 | 5m5.281s | 3.28 | 30.502s | 32.405s | 37.439s | 38.265s |
| `cache_hit` | 1000 | 100 | 1000 | 4m3.724s | 4.10 | 24.341s | 21.067s | 37.050s | 37.238s |
| `mixed_prompt` | 1000 | 100 | 1000 | 3m57.028s | 4.22 | 23.682s | 24.888s | 29.249s | 30.581s |

## Server Metrics

| Mode | Batches Total | Avg Batch Size | Avg Queue Wait (s) | Avg Execution (s) | Queue Rejected | Observed Requests |
|---|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 14743 | 4.88 | 0.0055 | 0.0131 | 0 | 1000 |
| `cache_hit` | 12188 | 5.83 | 0.0053 | 0.0125 | 0 | 1000 |
| `mixed_prompt` | 11623 | 6.19 | 0.0047 | 0.0128 | 0 | 1000 |

## Stage 2 Metrics

| Mode | Avg TTFT (s) | Avg TBT (s) | Prefix Hits | Prefix Misses | Prefix Tokens Saved |
|---|---:|---:|---:|---:|---:|
| `cache_miss` | 1.7322 | 0.4109 | 0 | 1000 | 0 |
| `cache_hit` | 0.3250 | 0.3430 | 1000 | 0 | 147000 |
| `mixed_prompt` | 1.2117 | 0.3209 | 0 | 1000 | 0 |

## 关键结论

### 1. Prefix cache metadata 明显降低 TTFT

相比 `cache_miss`，`cache_hit` 的平均 TTFT 从 `1.7322s` 降到 `0.3250s`。

降幅约为 `81%`。

这符合预期：prefix cache metadata 命中后，请求可以跳过大部分 prefill 工作，更快进入 decode 阶段。

### 2. Cache hit 提升吞吐并降低平均延迟

相比 `cache_miss`：

- throughput 从 `3.28 req/s` 提升到 `4.10 req/s`
- average latency 从 `30.502s` 降到 `24.341s`
- total batch count 从 `14743` 降到 `12188`
- average batch size 从 `4.88` 提升到 `5.83`

这说明减少 prefill 压力后，scheduler 更容易持续推进 decode work。

### 3. 当前 workload 的瓶颈不是 queue wait

三个场景中的平均 queue wait 都稳定在 `4.7ms` 到 `5.5ms` 左右。

主要延迟差异来自请求阶段行为：

- prefill cost 影响 TTFT
- 重复 decode step 影响 TBT 和端到端 latency
- cache hit 能减少 prefill 压力，但不会消除 decode loop 成本

### 4. Mixed prompt 比纯长 prompt cache miss 更快

`mixed_prompt` 没有 prefix cache 收益，但整体仍然快于 `cache_miss`。

这是合理的：`mixed_prompt` 包含 short 和 medium prompt，平均 prefill 成本低于纯 long prompt 的 `cache_miss` 场景。

### 5. Decode 是下一阶段更值得优化的瓶颈

即使 prefix cache 命中，在 `1000` 请求、`100` 并发下，平均延迟仍然较高。

原因是当前 Stage 2 模型每次 decode 只生成一个 token，这会产生大量 decode work item，并带来重复的 scheduler/executor 往返。

后续可以单独做 multi-token decode chunk 实验，但这不属于当前 Stage 2 收尾范围。

## Stage 2 Benchmark 总结

Stage 2 已经展示出一个最小 LLM-aware serving runtime 应具备的核心行为：

- prefill 和 decode 作为不同请求阶段是可观测的
- TTFT 和 TBT 能暴露不同延迟来源
- prefix cache metadata 可以被量化地降低 TTFT
- token-budget scheduling 能在 mixed prompt workload 下稳定运行且无 queue rejection
- benchmark metrics 已改为单次运行 delta，连续比较不同场景时更可靠
