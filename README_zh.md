# KVTide

<p align="center">
  <img src="./assets/banner.svg" alt="KVTide" width="520" />
</p>

<p align="center">
  <strong>从 runtime 出发，构建 KV-aware LLM serving。</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.26+" />
  <img src="https://img.shields.io/badge/Python-3.12%2B-3776AB?logo=python&logoColor=white" alt="Python 3.12+" />
  <img src="https://img.shields.io/badge/React-19-20232A?logo=react&logoColor=61DAFB" alt="React 19" />
  <img src="https://img.shields.io/badge/Connect-RPC-0B3954" alt="Connect RPC" />
  <img src="https://img.shields.io/badge/License-MIT-2E8B57" alt="MIT License" />
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

---

## KVTide 是什么？

KVTide 是一个将调度、请求状态和 KV cache ownership 显式化的开源 LLM serving runtime。Go 控制平面负责分词、请求生命周期、token-budget 调度、prefix cache 与 block metadata，并将结果流式返回；Python Executor 可以运行可控的 Mock workload，也可以执行真实的 Qwen Transformers CPU forward。

当前 runtime 刻意保持为**一个控制平面、一个 Executor 和一个 KV block pool**。这是研究 runtime 行为的一致性基线，而不是用普通服务副本伪装分布式推理。

> **Vision：** KV cache should move toward available compute automatically。KVTide 希望构建一个 Kubernetes-native runtime：兼容的 Executor 能够主动将可复用 KV block 推送到其他节点，并同步更新 ownership，避免后续请求重新计算 prefix。

## Runtime Console

Web 不是静态项目主页，而是 runtime 的交互控制台。

- **Demo / Topology** 通过 `GetExecutors` 发现连接中的 Executor，并展示 model、runtime epoch、device、dtype 和 KV capacity。
- **Demo / Request** 通过 Go 控制平面发送真实 Connect RPC 流式请求，并渲染生成的 Markdown。
- **Demo / Metrics** 读取 Prometheus metrics，解释 queue、batch、request、latency、prefix cache 和 block pool 状态。
- **Lab** 提供独立于真实 runtime 状态的逐步 token-budget 调度实验。

<p align="center">
  <img src="./assets/front-topology.png" alt="KVTide runtime topology" width="920" />
</p>

<p align="center">
  <img src="./assets/front-generate.png" alt="KVTide request demo" width="49%" />
  <img src="./assets/front-lab.png" alt="KVTide scheduler lab" width="49%" />
</p>

## 当前能力

| 方向 | 实现 |
|---|---|
| API | Connect RPC server-streaming inference、Executor RPC 和 Admin RPC |
| 请求生命周期 | queued、prefill、decode、streaming、finished、timeout 和 failed 状态迁移 |
| 调度 | prefill/decode 队列、chunked prefill、mixed work batch、sequence 与 token budget |
| Prefix cache | user-scoped cache salt、完整 block 的链式 hash、hit/miss 与 saved-token metrics |
| KV block | logical block table、free-list allocation、cache reuse、rollback、release 与 eviction 统计 |
| 执行 | Mock Runner 与 Qwen3-0.6B Transformers CPU Runner |
| KV tensor | Executor 侧 paged KV storage，以及 Hugging Face `DynamicCache` adapter |
| Streaming | 增量文本、TTFT/TBT 观测、usage、finish reason 与错误信息 |
| 可观测性 | Prometheus metrics 与 Executor runtime inventory |
| 部署 | Docker Compose、低资源 Cloud Mock profile，以及面向 kind 的 Kubernetes manifests |

## 架构

KVTide 将用户可见的请求与一次模型执行所需的调度任务分离：

- `Request` 管理输入 token、生命周期、生成结果、usage 和完成状态。
- `WorkItem` 描述一个可调度的 prefill chunk 或 decode step。
- `Scheduler` 在 sequence 与 token budget 下选择任务。
- `BlockManager` 负责 prefix matching 与逻辑 KV block metadata。
- `ExecutorManager` 将 batch 发送给已配置的 runtime。
- `Event` 提交或回滚 block 状态，并推进请求生命周期。

![KVTide 架构](./assets/Stage2_Architecture.svg)

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

控制平面会向 Executor 传递 token IDs、block table、新分配的 block IDs、computed-token offset 和 runtime epoch。Executor 使用这些 metadata 重建历史 KV、写入新 slot，并拒绝发送给旧 runtime instance 的任务。

## 快速开始

### 真实 Qwen CPU Executor

默认 Compose 使用 CPU 运行 Qwen3-0.6B。首先下载模型：

```bash
cd executor
uv run hf download Qwen/Qwen3-0.6B --local-dir ./models/Qwen3-0.6B
cd ..

docker compose up --build -d
```

访问 `http://127.0.0.1:5173`。

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

默认 Executor 使用 fp32 权重和 512 MiB KV cache budget，建议使用至少 8 GiB 内存的机器。

### 低资源 Cloud Mock

[`deploy/cloud`](./deploy/cloud) 使用一个 Mock Executor 启动完整拓扑，并且只暴露 Web 服务。该 profile 适合 2 核 2 GiB 等小型公开 Demo 服务器。

在服务器上导入已打包的镜像：

```bash
docker load -i kvtide-server.tar
docker load -i kvtide-executor.tar
docker load -i kvtide-web.tar

cd deploy/cloud
docker compose up -d
```

默认镜像 tag 是 `kvtide-server:local`、`kvtide-executor:local` 和 `kvtide-web:local`。需要时可以覆盖：

```bash
KVTIDE_SERVER_IMAGE=example/kvtide-server:v0.1 \
KVTIDE_EXECUTOR_IMAGE=example/kvtide-executor:v0.1 \
KVTIDE_WEB_IMAGE=example/kvtide-web:v0.1 \
KVTIDE_WEB_PORT=8080 \
docker compose up -d
```

默认访问地址是 `http://<server-ip>/`。Inference、Admin、metrics 和 Executor 端口只存在于 Compose 私有网络中。

## Benchmark

Benchmark 使用 Mock Executor，衡量的是控制平面行为，而不是 GPU kernel 性能。

![KVTide benchmark summary](./assets/Stage3_Benchmark_Summary.svg)

内置 profile 覆盖 cold prefix、已 warmup 的 user-scoped prefix、混合 prompt 长度、token-budget batching 和 KV block pressure。报告包含 throughput、平均与尾延迟、TTFT、TBT、batch size、prefix hits、saved tokens、allocation failures 和 evictions。

```bash
make bench-quick
make bench-report
```

历史报告保留在 [`docs/benchmarks`](./docs/benchmarks)。

## 使用 kind 部署 Kubernetes

Kubernetes manifests 保持相同的一对一 runtime 拓扑：

```bash
make docker-build
make kube-start
make kube-forward
```

Manifests、probe、rollout、检查命令和清理方式见 [`k8s/README.md`](./k8s/README.md)。

Executor Deployment 刻意保持一个 replica。在 Kubernetes Service 后添加 replica 只会负载均衡 batch，无法维持 Executor 本地 KV ownership。

## 范围与边界

当前已经实现：

- 可运行的 Go 调度与请求生命周期控制平面
- 真实流式 RPC 与 Prometheus 可观测性
- model-aware Go tokenizer
- 具备 Executor 侧 paged KV storage 的真实 CPU forward
- 单 Executor runtime 中的 prefix cache 与 KV block ownership metadata

尚未实现：

- CUDA、FlashAttention 或自定义 PagedAttention kernel
- tensor、pipeline 或 expert parallelism
- 在一次 GPU forward 中执行 mixed-phase batch
- multi-executor block namespace 与 placement policy
- peer-to-peer KV transfer
- 生产级 Kubernetes Operator 或 autoscaler
- 完整 OpenAI API 兼容

## Roadmap：从 Ownership 到 Mobility

下一个架构边界是 Executor-aware block ownership。只有先让每张 block table 归属于明确的 Executor 和 runtime epoch，控制平面才能在节点重启后安全恢复，或者将请求放置到多个副本。

在此基础上，KVTide 才能验证核心假设：当多个兼容 Executor 使用相同 model weights、dtype 和 tensor-parallel 配置时，KV 压力过高的 Executor 应该主动将部分 block **push** 给空闲节点。控制平面需要感知新的 placement、更新 metadata，并判断复用节省的计算是否大于传输成本。

这条路线需要实验数据，而不仅是实现功能。后续应对比 recomputation、local reuse 和 remote transfer 对 TTFT、TBT、throughput、transfer bandwidth、cache pressure 和 tail latency 的影响。

## 相关系统

- [vLLM](https://github.com/vllm-project/vllm)
- [SGLang](https://github.com/sgl-project/sglang)
- [LMCache](https://github.com/LMCache/LMCache)
- [TensorRT-LLM](https://github.com/NVIDIA/TensorRT-LLM)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)

## License

[MIT](./LICENSE)
