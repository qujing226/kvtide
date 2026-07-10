import asyncio

from connectrpc.request import RequestContext
from mini_llm_serve.v1 import core_pb2
from mini_llm_serve.v1 import execute_connect
from mini_llm_serve.v1 import execute_pb2
from runner.factory import create_runner
from setting import load_config

class ExecuteServiceImpl(execute_connect.ExecuteService):
    def __init__(self, runner):
        self.runner = runner

    async def execute_batch(self, request, ctx: RequestContext) -> execute_pb2.ExecuteBatchResponse:
        response = execute_pb2.ExecuteBatchResponse(
            batch_id=request.batch_id,
            executor_id="mock-python",
        )

        results = await asyncio.gather(
            *(self._execute_item(item) for item in request.items)
        )
        response.results.extend(results)
        return response

    async def _execute_item(self, item) -> execute_pb2.ExecuteResult:
        if item.phase == core_pb2.WORK_PHASE_PREFILL:
            return await self._execute_prefill(item)
        if item.phase == core_pb2.WORK_PHASE_DECODE:
            return await self._execute_decode(item)
        return execute_pb2.ExecuteResult(
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


    async def _execute_prefill(self, item):
        return await self.runner.prefill(item)

    async def _execute_decode(self, item):
        return await self.runner.decode(item)


app = execute_connect.ExecuteServiceASGIApplication(
    ExecuteServiceImpl(create_runner(load_config()))
)
