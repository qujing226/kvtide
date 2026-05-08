import asyncio
import random

from connectrpc.request import RequestContext

from mini_llm_serve.v1 import core_pb2
from mini_llm_serve.v1.execute_pb2 import (
    ExecuteBatchResponse,
    ExecuteResult,
)

from mini_llm_serve.v1.execute_connect import (
    ExecuteService,
    ExecuteServiceASGIApplication,
)

request_state: dict[str, int] = {}

MOCK_RESPONSE_TEXT = (
    "Currently, vLLM utilizes its own implementation of a multi-head query "
    "attention kernel (csrc/attention/attention_kernels.cu). This kernel is "
    "designed to be compatible with vLLM's paged KV caches, where the key and "
    "value cache are stored in separate blocks (note that this block concept "
    "differs from the GPU thread block. So in a later document, I will refer "
    'to vLLM paged attention block as "block", while refer to GPU thread '
    'block as "thread block").'
)
MOCK_RESPONSE_WORDS = MOCK_RESPONSE_TEXT.split()

class ExecuteServiceImpl(ExecuteService):
    async def execute_batch(self, request, ctx: RequestContext) -> ExecuteBatchResponse:
        response = ExecuteBatchResponse(
            batch_id = request.batch_id,
            executor_id = "mock-python",
        )

        results = await asyncio.gather(
            *(self._execute_item(item) for item in request.items)
        )
        response.results.extend(results)
        return response

    async def _execute_item(self, item) -> ExecuteResult:
        if item.phase == core_pb2.WORK_PHASE_PREFILL:
            return await self._execute_prefill(item)
        if item.phase == core_pb2.WORK_PHASE_DECODE:
            return await self._execute_decode(item)
        return ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=True,
            output_text="",
            finish_reason=core_pb2.FINISH_REASON_ERROR,
            computed_tokens=0,
            generated_tokens=0,
            execution_ms=0,
            error_message=f"unsupported work phase: {item.phase}."
        )
    
    async def _execute_prefill(self, item) -> ExecuteResult:
        scheduled_tokens = item.num_new_tokens or max(1, item.prompt_tokens)
        latency_ms = max(10, min(180, scheduled_tokens // 4 + random.randint(10, 30)))
        await asyncio.sleep(latency_ms / 1000)

        prompt_tokens = item.prompt_tokens or max(1, len(item.prompt) // 4)
        computed_tokens = min(prompt_tokens, item.computed_tokens + scheduled_tokens)
        if computed_tokens >= prompt_tokens:
            request_state[item.request_id] = 0

        return ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            output_text = "",
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=computed_tokens - item.computed_tokens,
            generated_tokens=0,
            execution_ms=latency_ms,
            error_message="",
        )

    async def _execute_decode(self, item) -> ExecuteResult:
        latency_ms = random.randint(5,20)
        await asyncio.sleep(latency_ms/1000)

        word_index = max(request_state.get(item.request_id, 0), item.generated_tokens)

        if word_index >= len(MOCK_RESPONSE_WORDS):
            request_state.pop(item.request_id, None)
            output_text = ""
            done = True
            generated_tokens = 0
            finish_reason = core_pb2.FINISH_REASON_STOP
        else:
            word = MOCK_RESPONSE_WORDS[word_index]
            output_text = word if word_index == 0 else f" {word}"
            word_index += 1
            done = word_index >= len(MOCK_RESPONSE_WORDS)
            generated_tokens = 1
            finish_reason = (
                core_pb2.FINISH_REASON_STOP
                if done
                else core_pb2.FINISH_REASON_UNSPECIFIED
            )

            if done:
                request_state.pop(item.request_id, None)
            else:
                request_state[item.request_id] = word_index

        return ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done = done,
            output_text=output_text,
            finish_reason = finish_reason,
            computed_tokens=0,
            generated_tokens=generated_tokens,
            execution_ms=latency_ms,
            error_message="",
        )


app = ExecuteServiceASGIApplication(ExecuteServiceImpl())

# def main():
    # print("Hello from llm-serve!")


# if __name__ == "__main__":
    # main()
