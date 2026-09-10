from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import TYPE_CHECKING

from kvtide.v1 import executor_pb2

if TYPE_CHECKING:
    from runtime.kv_transfer import KVTransferRuntime


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
    def kv_transfer(self) -> "KVTransferRuntime | None":
        """Mock/non-KV runners do not advertise a transferable cache."""
        return getattr(self, "_kv_transfer", None)

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

    @abstractmethod
    async def release_blocks(self, block_ids: list[int]) -> None:
        pass
