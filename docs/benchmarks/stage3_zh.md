# Stage 3 Benchmark 记录

[English Version](./stage3_en.md)

本文记录 Mini LLM Serve 在 Stage 3 阶段的 benchmark 结果。

Stage 3 在 serving control plane 模型上补充了：

- mock tokenizer 与 token ID
- KV block manager
- 基于完整 block hash 的 prefix cache
- free-list block reuse 与 cache eviction metrics
- quick regression 和 report generation 两种 benchmark profile

当前 executor 仍然是 Python mock backend。因此这些数字应该理解为 control-plane behavior，不是真实 GPU 推理性能。

![Stage 3 Benchmark Summary](../../assets/Stage3_Benchmark_Summary.svg)

## 测试设置

运行环境：

- client: `cmd/bench`
- server: Go inference service，监听 `:8800`
- metrics: Prometheus endpoint，监听 `:8801`
- backend: 通过 Connect RPC 调用 Python mock executor
- batch policy: token-budget mixed prefill/decode scheduling
- KV model: 临时设置为每个 block `16` tokens，总共 `1024` blocks

Benchmark profile：

- `bench-quick`：小规模确定性回归检查
- `bench-report`：本文使用的完整报告 profile

测试场景：

- `cache_miss`：1000 请求，并发 100，唯一用户
- `cache_hit`：1000 请求，并发 100，10 个已 warmup 的 cache user
- `mixed_prompt`：1000 请求，并发 100，混合 short/medium/long prompt
- `block_pressure`：320 请求，并发 32，长 prompt 制造 KV block pressure

## 场景结果

| Scenario | Requests | Concurrency | Success | Total | Throughput | Avg Latency | P50 | P90 | P99 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 1000 | 100 | 1000 | 3m47.141s | 4.40 req/s | 22.693s | 23.827s | 30.135s | 30.171s |
| `cache_hit` | 1000 | 100 | 1000 | 3m23.995s | 4.90 req/s | 20.380s | 19.873s | 25.492s | 27.818s |
| `mixed_prompt` | 1000 | 100 | 1000 | 3m23.775s | 4.91 req/s | 20.363s | 21.992s | 24.682s | 24.706s |
| `block_pressure` | 320 | 32 | 320 | 2m14.367s | 2.38 req/s | 13.428s | 13.003s | 15.403s | 16.538s |

## Scheduler Metrics

| Scenario | Batches | Avg Batch Size | Avg Queue Wait | Avg Execution | Queue Rejected | Observed Requests |
|---|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 13100 | 5.50 | 0.0055s | 0.0109s | 0 | 1000 |
| `cache_hit` | 12101 | 5.95 | 0.0054s | 0.0103s | 0 | 1000 |
| `mixed_prompt` | 11663 | 6.17 | 0.0053s | 0.0105s | 0 | 1000 |
| `block_pressure` | 6821 | 3.38 | 0.0055s | 0.0137s | 0 | 320 |

## Runtime Metrics

| Scenario | Avg TTFT | Avg TBT | Prefix Hits | Prefix Misses | Tokens Saved | Prefill Items | Decode Items | Cached Blocks | Allocation Failures | Evictions |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 1.6175s | 0.3010s | 0 | 1000 | 0 | 1000 | 71000 | 510 | 0 | 4490 |
| `cache_hit` | 0.8390s | 0.2791s | 1000 | 0 | 80000 | 1000 | 71000 | 50 | 0 | 460 |
| `mixed_prompt` | 1.3555s | 0.2715s | 0 | 1000 | 0 | 1000 | 71000 | 240 | 0 | 1475 |
| `block_pressure` | 2.1182s | 0.1615s | 0 | 320 | 0 | 320 | 22720 | 796 | 0 | 6164 |

每个场景结束后：

- active requests 回到 `0`
- inflight batches 回到 `0`
- active KV blocks 回到 `0`
- free KV blocks 回到 `1024`

`cached` blocks 和 `free` blocks 不是互斥状态。一个 cached block 可以处于 free list 中，同时仍然可以被后续 prefix match 复用。

## 结果解释

### Prefix cache 降低 prefill pressure

相比 `cache_miss`，`cache_hit` 将平均 TTFT 从 `1.6175s` 降到 `0.8390s`，降幅约 `48%`。

benchmark prompt 的 token 数超过了完整 cached blocks 覆盖的范围，因此每个正式请求仍然会为未缓存的尾部 token 产生一个 prefill work item。cache hit 并没有彻底消灭 prefill，而是降低了 prefill 成本。

因此 `cache_hit` 仍然会出现：

- `prefill_items = 1000`
- `prefix_hits = 1000`
- `tokens_saved = 80000`

真正重要的信号不是 “prefill work 为 0”，而是 cached prefix 让请求更快进入 decode，并降低 TTFT。

### Cache hit 改善 batch efficiency

相比 `cache_miss`，`cache_hit` 将 batch 数从 `13100` 降到 `12101`，并将 average batch size 从 `5.50` 提高到 `5.95`。

这说明 prefill pressure 降低后，scheduler 更容易形成更密的 mixed batch，也更容易持续推进 decode work。

### Mixed prompt 体现 workload composition 的影响

`mixed_prompt` 没有 prefix cache 收益，但吞吐仍达到 `4.91 req/s`，接近 cache-hit 场景。

原因是 workload composition：short 和 medium prompt 降低了平均 prefill 成本。因此它的 average batch size 为 `6.17`，高于纯 long prompt 的 `cache_miss` 场景 `5.50`。

这个场景说明 request count 不是好的调度信号。token cost 比 request count 更接近真实执行成本。

### Block pressure 暴露 KV churn

`block_pressure` 请求数和并发更低，但 prompt 更长。它将 average batch size 压低到 `3.38`，并将 evictions 提高到 `6164`。

这正是该场景要观察的信号：KV block pressure 会降低 batch efficiency，并制造更多 cache churn。该场景没有 queue rejection，也没有 allocation failure，因此本次运行测到的是压力状态，而不是系统过载失败。

### Queue wait 不是主要瓶颈

所有场景的 average queue wait 都稳定在 `5ms` 左右。

端到端延迟主要来自请求生命周期行为：

- prefill 影响 TTFT
- decode loop 影响 TBT 和 total latency
- cache hit 降低 prompt work，但不会消除 decode cost
- block pressure 降低 batch density，并增加 eviction churn

## 结论

- TTFT 和 TBT 必须分开统计，因为 prefill pressure 和 decode-loop behavior 的原因不同。
- Prefix cache 应该通过 saved tokens 和 TTFT 来解释，而不是只看 hit count。
- Token-aware scheduling 是必要的，因为 request count 不能表达执行成本。
- KV block pressure 可以通过 average batch size、cached/free block state、evictions 和 allocation failures 观测。
- 即使使用 mock executor，只要指标绑定 request lifecycle 和 scheduler decision，仍然可以有效暴露 serving control-plane behavior。
