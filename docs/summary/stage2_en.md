# Stage 2 Report

[中文版本](./stage2_zh.md)

## Summary

Stage 2 evolves `mini-llm-serve` from a request-level batching system into a more LLM-aware scheduling playground.

In Stage 1, the central unit was a request. A request entered a FIFO queue, was batched, and was sent to the Python mock executor. This was enough to validate the serving pipeline, but it could not express the most important differences in LLM inference: prefill and decode have different costs, prompt length affects scheduling pressure, streaming needs separate first-token and between-token metrics, and prefix cache hits can change prefill cost.

The core change in Stage 2 is the introduction of LLM-aware internal models. The system now explicitly separates `Request`, `WorkItem`, and `Event`:

- `Request` represents the full lifecycle and state of a user request.
- `WorkItem` represents one schedulable execution task, either prefill or decode.
- `Event` represents executor results and drives the state machine forward.

This split moves the system from “batching requests” to “scheduling token-aware work”. It also gives prefill/decode separation, token-budget scheduling, streaming metrics, and prefix cache metadata a natural place in the architecture.

## Key Takeaways

- Stage 2 introduces an explicit prefill / decode execution model.
- The scheduler is no longer based only on request count. It now uses token budget constraints.
- Request lifecycle is managed by a state machine. Executor events generate the next `WorkItem`.
- The streaming path exposes TTFT and TBT, separating first-token latency from between-token latency.
- Prefix cache metadata changes prefill cost and measurably reduces TTFT in benchmarks.
- The system is still mock inference. It does not implement real KV cache, PagedAttention, or GPU kernels.

## Architecture Changes

Stage 2 keeps the split between the Go control plane and the Python mock executor, but changes the core Go-side abstractions.

![Stage 2 Architecture](../../assets/Stage2_Architecture.svg)

In Stage 1, a request was mostly scheduled as a single unit. In Stage 2, the request is first owned by `RequestLifecycleStateManager`, then converted into schedulable `WorkItem`s:

```text
GenerateRequest
  -> Request
  -> initial WorkItem
  -> Scheduler
  -> ExecutorManager
  -> Python Mock Executor
  -> Event
  -> RequestLifecycleStateManager
  -> next WorkItem or final response
```

This loop is the center of Stage 2. The scheduler does not directly decide when a request is complete. It chooses the next executable work. Whether a request continues prefill, enters decode, keeps decoding, or finishes is decided by the state machine based on executor events.

This separates three responsibilities:

- handler / transport owns request ingress and response delivery
- scheduler owns token-budget-based work selection
- state manager owns request lifecycle and event progression

## Request Lifecycle

The Stage 2 request lifecycle can be summarized as:

![Stage 2 Request Lifecycle](../../assets/Stage2_Request_Lifecycle.svg)

```text
queued
  -> prefill ready
  -> prefill running
  -> decode ready
  -> decode running
  -> finished
```

Exceptional paths include:

```text
timeout
canceled
failed
```

When a request is created, the system estimates prompt tokens and checks prefix cache metadata. If there is no cache hit, the request starts with prefill work. If the cache covers the full prompt, the request can skip prefill and start directly with decode work.

After prefill finishes, the state machine stores prefix cache metadata and creates the first decode work item. Every decode event updates generated token count and decides whether the request should continue decoding or finish due to stop, length, or error.

The value of this model is that it exposes the phase structure of LLM inference to the serving system. The system no longer only knows that “the request is not finished”. It can know whether the request is in prefill, in decode, has emitted its first token, and whether another decode step should be scheduled.

## Prefill / Decode Separation

Stage 2 models prefill and decode as different types of work.

Prefill work is costed by prompt tokens. It represents reading the prompt and building context. In a real system, it is usually more compute-heavy. Decode work currently emits one token per step. In a real system, it is usually latency-sensitive and repeatedly re-enters the scheduler.

This split enables two important behaviors:

- the scheduler can prioritize decode and avoid making streaming requests wait too long
- benchmarks can observe TTFT and TBT separately instead of only measuring end-to-end latency

The current implementation still emits one token per decode step. This keeps behavior easy to observe, but it also amplifies scheduler and executor RPC round-trip cost. Multi-token decode chunks are left as a later experiment.

## Token Budget Scheduler

Stage 1 batching was mainly controlled by request count and timeout. Stage 2 adds token budget constraints.

During each scheduling round, the scheduler considers:

- `maxBatchSeqs`: maximum number of work items in one batch
- `maxBatchTokens`: maximum token budget consumed by one batch
- decode work: currently costed as 1 token
- prefill work: costed by the number of tokens to prefill in this round
- small / large prefill queues: separated by prompt token threshold
- partial prefill: large prefill work can be split into a chunk when remaining budget is limited

This model is closer to real serving systems because it recognizes that “one request” and “one request’s execution cost” are not the same thing.

In the current implementation, the scheduler takes decode work first. If there is no decode pressure, it relaxes prefill limits so the batch can be filled with prefill work. This preserves streaming priority while avoiding poor batch utilization when the workload is prefill-only.

## Streaming and TTFT / TBT

Stage 2 adds streaming-oriented observability:

- `TTFT`: time to first token
- `TBT`: time between tokens

These metrics must be separated because they come from different system causes:

- TTFT is mainly affected by queueing, prefill, and prefix cache hits
- TBT is mainly affected by decode scheduling, executor execution, event routing, and streaming response delivery

If the system only reports end-to-end latency, prefill and decode problems are mixed together. Stage 2 metrics make it possible to answer a more specific question: is the first token slow, or are later tokens slow?

## Prefix Cache Metadata

Stage 2 does not implement real KV cache or PagedAttention. It implements prefix cache metadata.

The metadata records how many prompt tokens have already been prefetched for a given `cache_key`. When a new request carries the same `cache_key`, the state manager looks up the cached token count and applies it to `ComputedTokens`.

If the cache covers the full prompt, the request skips prefill and goes directly to decode. If only part of the prompt is covered, the system creates prefill work only for the remaining tokens.

This is not GPU KV cache management, but it captures the key serving-control-plane semantics of cache-aware scheduling:

- cache hits reduce prefill cost
- cache hits reduce TTFT
- cache hit / miss behavior is observable
- saved prefix tokens quantify the cache benefit

## Benchmark Results

Stage 2 benchmark uses three scenarios:

![Stage 2 Benchmark Summary](../../assets/Stage2_Benchmark_Summary.svg)

- `cache_miss`: all requests use unique cache keys
- `cache_hit`: one warmup request builds a shared prefix cache entry before the measured run
- `mixed_prompt`: short, medium, and long prompts are mixed, all with unique cache keys

Key results:

| Mode | Throughput (req/s) | Avg Latency | Avg TTFT (s) | Avg TBT (s) | Prefix Hits | Prefix Misses | Tokens Saved |
|---|---:|---:|---:|---:|---:|---:|---:|
| `cache_miss` | 3.28 | 30.502s | 1.7322 | 0.4109 | 0 | 1000 | 0 |
| `cache_hit` | 4.10 | 24.341s | 0.3250 | 0.3430 | 1000 | 0 | 147000 |
| `mixed_prompt` | 4.22 | 23.682s | 1.2117 | 0.3209 | 0 | 1000 | 0 |

The most important result is that prefix cache metadata significantly reduces TTFT. Compared with `cache_miss`, `cache_hit` reduces average TTFT from `1.7322s` to `0.3250s`, an approximate `81%` reduction.

For detailed benchmark tables and observations, see [Stage 2 Benchmark Notes](../benchmarks/stage2_en.md).

## Limitations

Stage 2 still has clear boundaries:

- the executor is a Python mock executor, not a real model runtime
- prefix cache is metadata, not real KV block cache
- PagedAttention, FlashAttention, and GPU memory allocation are not implemented
- decode currently emits one token per step, amplifying scheduling and RPC round-trip cost
- batch metrics are still mostly observed as mixed batches, and can be further split into prefill/decode metrics later

These limits are intentional. The goal of Stage 2 is not to copy vLLM. The goal is to establish the core control-plane model for LLM-aware scheduling in a small, readable, runnable Go serving system.

## Stage 2 Summary

After Stage 2, `mini-llm-serve` is no longer only a dynamic batching demo.

It now has the key abstractions expected from a minimal LLM serving control plane:

- request lifecycle state machine
- prefill / decode separation
- token budget scheduler
- streaming output with TTFT / TBT metrics
- prefix cache metadata
- executor manager and event-driven result routing
- reproducible benchmarks and system behavior analysis

The next most valuable directions are not just adding more features. They are:

- running the full service in a local or Kubernetes environment to build deployment, routing, and production-system understanding
- experimenting with multi-token decode chunks to observe changes in TBT, latency, and throughput
