import secrets

from connectrpc.code import Code
from connectrpc.errors import ConnectError
from connectrpc.request import RequestContext

from mini_llm_serve.v1 import executor_connect, executor_pb2
from runner import ModelRunner
from setting import ExecutorConfig


class ExecuteServiceImpl(executor_connect.ExecutorService):
    def __init__(self, runner: ModelRunner, cfg: ExecutorConfig):
        self.runner = runner
        self.cfg = cfg
        # Avoid the proto3 default value so an omitted epoch never looks valid.
        self.runtime_epoch = secrets.randbelow(2**32 - 1) + 1

    async def execute_batch(
        self, request, ctx: RequestContext
    ) -> executor_pb2.ExecuteBatchResponse:
        self._validate_epoch(request.runtime_epoch)
        response = executor_pb2.ExecuteBatchResponse(
            batch_id=request.batch_id,
            executor_id=self.cfg.runner.executor_id,
        )
        response.results.extend(await self.runner.execute(list(request.items)))
        return response

    async def get_runtime(
        self, request: executor_pb2.GetRuntimeRequest, ctx: RequestContext
    ) -> executor_pb2.GetRuntimeResponse:
        info = self.runner.runtime_info
        return executor_pb2.GetRuntimeResponse(
            executor_id=self.cfg.runner.executor_id,
            runtime_epoch=self.runtime_epoch,
            model_id=self.cfg.runner.model_id,
            model_type=info.model_type,
            dtype=info.dtype,
            device_type=self.cfg.runtime.device,
            tensor_parallel_size=self.cfg.runtime.tensor_parallel_size,
            block_size=info.block_size,
            num_kv_blocks=info.num_kv_blocks,
            num_hidden_layers=info.num_hidden_layers,
            num_kv_heads=info.num_kv_heads,
            head_dim=info.head_dim,
            total_memory_bytes=info.total_memory_bytes,
            available_memory_bytes=info.available_memory_bytes,
            kv_cache_bytes=info.kv_cache_bytes,
        )

    async def release_blocks(
        self, request: executor_pb2.ReleaseBlocksRequest, ctx: RequestContext
    ) -> executor_pb2.ReleaseBlocksResponse:
        self._validate_epoch(request.runtime_epoch)
        await self.runner.release_blocks(list(request.block_ids))
        return executor_pb2.ReleaseBlocksResponse()

    def _validate_epoch(self, epoch: int) -> None:
        if epoch != self.runtime_epoch:
            raise ConnectError(
                Code.FAILED_PRECONDITION,
                f"runtime epoch mismatch: got {epoch}, want {self.runtime_epoch}",
            )
