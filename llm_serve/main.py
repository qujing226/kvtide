import asyncio
import random

import request_state.request as state

from connectrpc.request import RequestContext

from mini_llm_serve.v1 import core_pb2
from mini_llm_serve.v1 import execute_pb2
from mini_llm_serve.v1 import execute_connect 



class ExecuteServiceImpl(execute_connect.ExecuteService):
    async def execute_batch(self, request, ctx: RequestContext) -> execute_pb2.ExecuteBatchResponse:
        response = execute_pb2.ExecuteBatchResponse(
            batch_id = request.batch_id,
            executor_id = "mock-python",
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
            output_text="",
            finish_reason=core_pb2.FINISH_REASON_ERROR,
            computed_tokens=0,
            generated_tokens=0,
            execution_ms=0,
            error_message=f"unsupported work phase: {item.phase}."
        )
    
    async def _execute_prefill(self, item) -> execute_pb2.ExecuteResult:
        scheduled_tokens = item.num_new_tokens or max(1, item.prompt_tokens)
        latency_ms = max(10, min(180, scheduled_tokens // 4 + random.randint(10, 30)))
        await asyncio.sleep(latency_ms / 1000)

        prompt_tokens = item.prompt_tokens or max(1, len(item.prompt) // 4)
        computed_tokens = min(prompt_tokens, item.computed_tokens + scheduled_tokens)
        if computed_tokens >= prompt_tokens:
            state.request_state[item.request_id] = 0

        return execute_pb2.ExecuteResult(
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

    async def _execute_decode(self, item) -> execute_pb2.ExecuteResult:
        latency_ms = random.randint(5,20)
        await asyncio.sleep(latency_ms/1000)

        word_index = max(state.request_state.get(item.request_id, 0), item.generated_tokens)

        if word_index >= len(state.MOCK_RESPONSE_WORDS):
            state.request_state.pop(item.request_id, None)
            output_text = ""
            done = True
            generated_tokens = 0
            finish_reason = core_pb2.FINISH_REASON_STOP
        else:
            word = state.MOCK_RESPONSE_WORDS[word_index]
            output_text = word if word_index == 0 else f" {word}"
            word_index += 1
            done = word_index >= len(state.MOCK_RESPONSE_WORDS)
            generated_tokens = 1
            finish_reason = (
                core_pb2.FINISH_REASON_STOP
                if done
                else core_pb2.FINISH_REASON_UNSPECIFIED
            )

            if done:
                state.request_state.pop(item.request_id, None)
            else:
                state.request_state[item.request_id] = word_index

        return execute_pb2.ExecuteResult(
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


app = execute_connect.ExecuteServiceASGIApplication(ExecuteServiceImpl())

# def main():
    # print("Hello from llm-serve!")


# if __name__ == "__main__":
    # main()
