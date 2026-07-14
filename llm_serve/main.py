import asyncio
import secrets

from connectrpc.request import RequestContext
from connectrpc.code import Code
from connectrpc.errors import ConnectError
from mini_llm_serve.v1 import core_pb2
from mini_llm_serve.v1 import executor_connect
from mini_llm_serve.v1 import executor_pb2
from runner.factory import create_runner
from setting import load_config, ExecutorConfig

from runner import ModelRunner


class ExecuteServiceImpl(executor_connect.ExecutorService):
    def __init__(self, runner: ModelRunner, cfg: ExecutorConfig):
        self.runner = runner
        self.cfg = cfg
        # plus one to avoid the default value "0" in grpc
        self.runtime_epoch = secrets.randbelow(2**32 - 1) + 1

    async def execute_batch(
        self, request, ctx: RequestContext
    ) -> executor_pb2.ExecuteBatchResponse:
        self._validate_epoch(request.runtime_epoch)
        response = executor_pb2.ExecuteBatchResponse(
            batch_id=request.batch_id,
            executor_id=self.cfg.runner.executor_id,
        )

        results = await asyncio.gather(
            *(self._execute_item(item) for item in request.items)
        )
        response.results.extend(results)
        return response

    async def _execute_item(self, item) -> executor_pb2.ExecuteResult:
        if item.phase == core_pb2.WORK_PHASE_PREFILL:
            return await self.runner.prefill(item)
        if item.phase == core_pb2.WORK_PHASE_DECODE:
            return await self.runner.decode(item)
        return executor_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=True,
            token_id=0,
            finish_reason=core_pb2.FINISH_REASON_ERROR,
            computed_tokens=0,
            generated_tokens=0,
            execution_ms=0,
            error_message=f"unsupported work phase: {item.phase}.",
        )

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


cfg = load_config()
runner = create_runner(cfg)
app = executor_connect.ExecutorServiceASGIApplication(ExecuteServiceImpl(runner, cfg))
