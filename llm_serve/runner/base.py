from abc import ABC, abstractmethod
from dataclasses import dataclass
from mini_llm_serve.v1 import executor_pb2


@dataclass(frozen=True, slots=True)
class RuntimeInfo:
    model_type: str
    dtype: str

    block_size: int
    num_kv_blocks: int

    num_hidden_layers: int
    num_kv_heads: int
    head_dim: int

    total_memory_bytes: int
    available_memory_bytes: int
    kv_cache_bytes: int


class ModelRunner(ABC):
    @property
    @abstractmethod
    def runtime_info(self) -> RuntimeInfo:
        pass

    @abstractmethod
    async def execute(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        """Execute the scheduler batch, ideally with one mixed-phase forward."""
        pass

    # These phase-specific methods are transitional building blocks. The target
    # execution path keeps the original mixed batch intact through execute().
    @abstractmethod
    async def prefill(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        pass

    @abstractmethod
    async def decode(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        pass

    # Single-item helpers remain available for focused tests and compatibility.
    async def prefill_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        results = await self.prefill([item])
        return results[0]

    async def decode_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        results = await self.decode([item])
        return results[0]

    @abstractmethod
    async def release_blocks(self, block_ids: list[int]) -> None:
        pass
