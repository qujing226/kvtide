from abc import ABC, abstractmethod
from mini_llm_serve.v1 import execute_pb2


class ModelRunner(ABC):
    @abstractmethod
    async def prefill(self, item) -> execute_pb2.ExecuteResult:
        pass

    @abstractmethod
    async def decode(self, item) -> execute_pb2.ExecuteResult:
        pass
