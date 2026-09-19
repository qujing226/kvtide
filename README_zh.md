# KVTide

<p align="center">
  <img src="./assets/banner.svg" alt="KVTide" width="520" />
</p>

<p align="center">
  <strong>一个探索 KV ownership、prefix reuse 与跨 Executor 状态迁移的 LLM serving 研究原型。</strong>
</p>

<p align="center">
  <a href="./README.md">English</a>
  ·
  <a href="#快速开始">快速开始</a>
  ·
  <a href="#架构">架构</a>
  ·
  <a href="./k8s/README.md">Kubernetes</a>
</p>

> **项目状态：已冻结。** KVTide 已完成当前研究探索，不再作为活跃论文项目或生产 serving system 继续开发。仓库保留为可运行的实验原型和工程作品；这里没有尚待完成的 placement policy roadmap。

## KVTide 是什么？

KVTide 将 LLM serving 中通常隐含在 runtime 内部的状态显式化：

- Go Engine 管理 tokenization、请求生命周期、调度、Executor runtime 信息和 KV block metadata。
- Python Executor 管理模型执行、设备上的 paged KV tensor，以及 KV block 的导出和导入。
- 每个 Executor 拥有独立的 block table、队列和 runtime epoch。
- Engine 可以在兼容的 Executor 之间执行一次显式的 KV replication transaction。

项目最初研究“是否应当主动把热门 prefix KV 推向空闲算力”。随着相关系统的边界逐渐清晰，这个问题本身不足以支撑继续扩展一套独立 runtime。KVTide 因此停在机制正确性原型，而不是继续实现预测策略、完整推理内核或生产控制面。研究过程与停止依据保留在 [`RESEARCH_QUESTION.md`](./RESEARCH_QUESTION.md)。

## 已实现能力

| 模块 | 当前实现 |
|---|---|
| 请求与调度 | 流式请求生命周期、prefill/decode WorkItem、chunked prefill、mixed work batch、sequence/token budget |
| 多 Executor | Executor runtime discovery、按 Executor 分离的工作队列和 block registry |
| Prefix cache | user-scoped cache salt、完整 block 的链式 hash、prefix match、引用与释放 |
| KV block lifecycle | allocation、reservation、commit、rollback、cache retention 和 release |
| 模型执行 | Mock Runner 与基于 Transformers 的 Qwen causal-LM Runner，支持 CPU 和 CUDA |
| Paged KV | Executor 侧 paged K/V tensor、物理 slot 写入与 Hugging Face `DynamicCache` adapter |
| KV transfer | Engine prepare → source push → destination import → Engine commit/rollback |
| 兼容性与防陈旧 | runtime epoch、实际加载权重 fingerprint、KV layout version、dtype/geometry/config compatibility fingerprint |
| 接口与观测 | Connect RPC、增量输出、Prometheus metrics 与 runtime inventory |
| 部署 | Docker Compose、Web runtime console 和 kind manifests |

## 架构

![KVTide architecture](./assets/Architecture.svg)

请求执行路径：

```text
client
  -> Go Engine
     -> request state machine
     -> executor-scoped scheduler
     -> executor-scoped block registry
     -> ExecuteBatch
  -> Python Executor
     -> model forward
     -> local paged KV tensors
  -> event / streamed result
```

KV replication 路径：

```text
Engine Coordinator
  -> reserve destination blocks and pin source blocks
  -> TriggerKVPush(source)
  -> source exports selected complete blocks
  -> PushKV(destination)
  -> destination validates identity, epoch and compatibility
  -> destination imports K/V and confirms device completion
  -> Engine commits destination prefix metadata

failure
  -> Engine rollback
  -> release source pins and destination reservations
```

Prefix hash 由 Engine 生成并随 transaction 传递；Executor 不重新推导 prefix identity。物理 block ID 始终是 Executor-local 的，目标端使用预留的本地 block ID 接收数据。

## 已验证范围

仓库中的 Go 和 Python 单元测试覆盖 scheduler、block lifecycle、runtime epoch、兼容性校验、KV tensor round-trip、RPC transfer、失败回滚和 inference reuse。

开发期间还在一张 RTX 4090 D 上启动了两个独立 Executor 进程，验证了以下完整路径：

```text
source prefill
  -> Engine-triggered KV push
  -> destination prefix reuse
  -> 与 destination 本地完整 prefill 的输出一致性比较
```

这次一次性实验用于验证数据流和正确性，不是性能 benchmark，也不构成多 GPU 或多节点扩展性结论。

## 限制

KVTide 是研究原型，不是 vLLM、SGLang 或 LMCache 的替代品。

- KV transfer v1 将 tensor 同步到 host，编码为完整 raw protobuf bytes，再由目标端复制到设备；它不是 GPU Direct、RDMA 或 NIXL 路径。
- 每个 Executor 使用一个 cache lock 串行化 inference、release、snapshot 和 import，没有并发 CUDA stream overlap。
- 单次 KV transfer 默认上限为 64 MiB，不支持压缩、分块流式传输或增量重试。
- KV compatibility 要求相同的实际权重、模型配置、dtype、KV geometry、layout version 和 tensor-parallel size。
- KV transfer v1 只支持 `tensor_parallel_size=1` 和从位置 0 开始的完整 prefix blocks。
- 没有 placement policy、需求预测、autoscaling 或 proactive replication controller。
- 没有 FlashAttention/PagedAttention 自定义 kernel、tensor/pipeline/expert parallelism，也没有生产级故障恢复。
- 已验证的是机制正确性，不是吞吐、tail latency 或成本优势。

## 快速开始

### 下载模型并启动

默认 Compose 使用 Qwen3-0.6B：

```bash
cd executor
uv run hf download Qwen/Qwen3-0.6B --local-dir ./models/Qwen3-0.6B
cd ..

docker compose up --build -d
```

| 服务 | 地址 |
|---|---|
| Web runtime console | `http://127.0.0.1:5173` |
| Inference API | `http://127.0.0.1:8800` |
| Admin API 与 metrics | `http://127.0.0.1:8801` |

```bash
docker compose ps
docker compose logs -f
curl http://127.0.0.1:8801/metrics
docker compose down
```

### 测试

```bash
go test ./...

cd executor
uv run python -m unittest discover -s tests -v
```

GPU 环境可能需要使用与宿主驱动兼容的 PyTorch wheel。已有环境不希望触发 `uv` 重新解析依赖时，可以显式使用 `uv run --no-sync`；这不是仓库默认开发命令。

## Benchmark 与 Kubernetes

`make bench-quick` 和 `make bench-report` 使用 Mock Executor 测量控制平面行为，不代表 GPU kernel 性能。历史报告保留在 [`docs/benchmarks`](./docs/benchmarks)。

kind 部署命令和 manifests 见 [`k8s/README.md`](./k8s/README.md)。这些 manifests 用于展示服务拓扑，不代表生产级 Operator 或 autoscaling 支持。

## 冻结边界

KVTide 不再计划继续实现：

- proactive KV placement policy；
- 通用 cross-model KV transformation middleware；
- Go 版完整推理引擎；
- GPU Direct、多节点传输或生产级 Kubernetes 控制面。

未来如果从成熟 serving 系统中出现了一个新的、可被实验否证的问题，可以把本仓库作为 research harness 使用；这不是当前 roadmap，也不构成维护承诺。

## 相关系统

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [LMCache](https://github.com/LMCache/LMCache)
- [llm-d](https://github.com/llm-d/llm-d)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)

## License

[MIT](./LICENSE)
