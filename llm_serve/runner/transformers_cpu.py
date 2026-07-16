import time
from typing import Protocol
import psutil

import torch
from transformers import AutoConfig, AutoModelForCausalLM
from mini_llm_serve.v1 import core_pb2, executor_pb2
from runner.base import ModelRunner, RuntimeInfo
from adapter import DynamicCacheAdapter
from runtime import BatchBuilder, PagedKVCache

KV_CACHE_BLOCK_SIZE = 16


class QwenRunnerConfig(Protocol):
    @property
    def model_path(self) -> str: ...
    @property
    def dtype(self) -> str: ...


class QwenTransformersRunner(ModelRunner):
    def __init__(
        self,
        cfg: QwenRunnerConfig,
        kv_cache_memory_bytes: int,
    ):
        """
        kv_cache_memory_bytes is the limitation of memory utility. e.g. 256 * 1024 (Byte)
        """
        if kv_cache_memory_bytes <= 0:
            raise ValueError("kv_cache_memory_bytes must be positive")

        self.cfg = cfg
        self.config = AutoConfig.from_pretrained(
            cfg.model_path,
            trust_remote_code=True,
        )
        self.dtype = resolve_torch_dtype(cfg.dtype)

        self.model = AutoModelForCausalLM.from_pretrained(
            cfg.model_path,
            dtype=self.dtype,
            device_map=None,
            trust_remote_code=True,
        )
        self.model.eval()

        self.mem = psutil.virtual_memory()
        if kv_cache_memory_bytes > self.mem.available:
            raise ValueError(
                "KV cache memory budget exceeds available system memory: "
                f"budget={kv_cache_memory_bytes}, "
                f"available={self.mem.available}"
            )

        self.block_size = KV_CACHE_BLOCK_SIZE
        self.num_layers = self.config.num_hidden_layers
        self.num_kv_heads = self.config.num_key_value_heads
        self.head_dim = getattr(
            self.config,
            "head_dim",
            self.config.hidden_size // self.config.num_attention_heads,
        )

        dtype_bytes = torch.empty(
            (),
            dtype=self.dtype,
        ).element_size()

        self.block_bytes = (
            2
            * self.num_layers
            * self.block_size
            * self.num_kv_heads
            * self.head_dim
            * dtype_bytes
        )

        self.num_kv_blocks = kv_cache_memory_bytes // self.block_bytes
        if self.num_kv_blocks < 1:
            raise ValueError("KV cache memory budget cannot hold one block")

        self.batch_builder = BatchBuilder(self.block_size)

        self.kv_cache = PagedKVCache(
            num_layers=self.num_layers,
            num_blocks=self.num_kv_blocks,
            block_size=self.block_size,
            num_kv_heads=self.num_kv_heads,
            head_dim=self.head_dim,
            dtype=self.dtype,
            device="cpu",
        )

        self.cache_adapter = DynamicCacheAdapter(
            paged_cache=self.kv_cache,
            model_config=self.config,
        )

        self._runtime_info = RuntimeInfo(
            model_type=self.config.model_type,
            dtype=str(self.dtype).removeprefix("torch."),
            block_size=self.block_size,
            num_kv_blocks=self.num_kv_blocks,
            num_hidden_layers=self.num_layers,
            num_kv_heads=self.num_kv_heads,
            head_dim=self.head_dim,
            total_memory_bytes=self.mem.total,
            available_memory_bytes=self.mem.available,
            kv_cache_bytes=self.kv_cache.cache_bytes,
        )

        self.eos_token_ids = normalize_eos_token_ids(self.config.eos_token_id)

    async def execute(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        decode_items = [
            item for item in items if item.phase == core_pb2.WORK_PHASE_DECODE
        ]
        prefill_items = [
            item for item in items if item.phase == core_pb2.WORK_PHASE_PREFILL
        ]
        return await self.decode(decode_items) + await self.prefill(prefill_items)

    async def prefill(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        return [await self.prefill_one(item) for item in items]

    async def decode(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        return [await self.decode_one(item) for item in items]

    async def prefill_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        start_time = time.perf_counter()

        execution_ms = int((time.perf_counter() - start_time) * 1000)
        return executor_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            token_id=0,
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=len(item.token_ids),
            generated_tokens=0 if not item.sample else 1,
            execution_ms=execution_ms,
            error_message="",
        )

    async def decode_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        start_time = time.perf_counter()

        input_ids = torch.tensor([list(item.token_ids)], dtype=torch.long)

        with torch.inference_mode():
            outputs = self.model(input_ids=input_ids)
            logits = outputs.logits[:, -1, :]
            next_token_id = int(torch.argmax(logits, dim=-1).item())

        execution_ms = int((time.perf_counter() - start_time) * 1000)

        done = next_token_id in self.eos_token_ids

        finish_reason = (
            core_pb2.FINISH_REASON_STOP if done else core_pb2.FINISH_REASON_UNSPECIFIED
        )

        return executor_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=done,
            token_id=next_token_id,
            finish_reason=finish_reason,
            computed_tokens=0,
            generated_tokens=1,
            execution_ms=execution_ms,
            error_message="",
        )

    async def release_blocks(self, block_ids: list[int]) -> None:
        self.kv_cache.release(block_ids)

    @property
    def runtime_info(self) -> RuntimeInfo:
        return self._runtime_info


def normalize_eos_token_ids(eos):
    if eos is None:
        return set()
    if isinstance(eos, int):
        return {eos}
    return set(eos)


def resolve_torch_dtype(dtype: str) -> torch.dtype:
    match dtype:
        case "float32" | "fp32":
            return torch.float32
        case "float16" | "fp16":
            return torch.float16
        case "bfloat16" | "bf16":
            return torch.bfloat16
        case _:
            raise ValueError(f"unsupported dtype: {dtype}")


# def get_memory_info():
#     # psutil.virtual_memory() 返回一个命名元组，包含了极其丰富的内存指标
#     mem = psutil.virtual_memory()
#     print(f"总内存 (Total):   {mem.total} Bytes ({mem.total / (1024**3):.2f} GB)")
#     print(
#         f"可用内存 (Available): {mem.available} Bytes ({mem.available / (1024**3):.2f} GB)"
#     )
#     print(f"已用内存 (Used):    {mem.used} Bytes ({mem.used / (1024**3):.2f} GB)")
#     print(f"内存使用率:         {mem.percent}%")
