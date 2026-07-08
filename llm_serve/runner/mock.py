import asyncio
import random

from block import block
from mini_llm_serve.v1 import core_pb2, execute_pb2
from request_state import request_state
from runner.base import ModelRunner


class MockRunner(ModelRunner):
    async def prefill(self, item: execute_pb2.ExecuteItem) -> execute_pb2.ExecuteResult:
        # Refresh the executor-side KV metadata shadow from the control plane.
        kv_state = block.update_runtime(item)

        # Tokens scheduled for this prefill chunk.
        # In the normal path, this is the remaining prompt tokens selected by the scheduler.
        scheduled_tokens = item.num_new_tokens or len(item.token_ids) or 1

        latency_ms = prefill_latency_ms(
            scheduled_tokens,
            random.randint(30, 70),
            random.randint(300, 800),
        )
        await asyncio.sleep(latency_ms / 1000)

        # Tokens actually computed by this prefill chunk.
        # The Go control plane owns lifecycle transition, so the executor returns only a delta.
        computed_delta = min(scheduled_tokens, len(item.token_ids) or scheduled_tokens)

        kv_state.computed_tokens += computed_delta
        block.set_runtime(item.request_id, kv_state)

        return execute_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            token_id=random.randint(1, 200),
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=computed_delta,
            generated_tokens=0,
            execution_ms=latency_ms,
            error_message="",
        )

    async def decode(self, item: execute_pb2.ExecuteItem) -> execute_pb2.ExecuteResult:
        # Refresh the executor-side KV metadata shadow for this decode step.
        kv_state = block.update_runtime(item)

        block_table_size = len(kv_state.block_table)
        # In decode phase, one decode item usually won't take a new slot.
        new_blocks = len(item.kv_blocks.allocated_blocks) or 0

        # Decode reads existing KV cache blocks to generate the next token.
        # Longer block tables approximate higher memory-access cost.
        latency_ms = decode_latency_ms(
            block_table_size,
            new_blocks,
            random.randint(70, 120),
        )
        await asyncio.sleep(latency_ms / 1000)

        # Use the larger progress value to avoid generating duplicate words if
        # either the mock runtime state or control-plane state is behind.
        word_index = max(request_state.get_decode_index(item.request_id), item.generated_tokens)

        if word_index >= len(request_state.MOCK_RESPONSE_CHUNKS):
            clear_request(item.request_id)
            done = True
            generated_tokens = 0
            finish_reason = core_pb2.FINISH_REASON_STOP
        else:
            word_index += 1
            done = word_index >= len(request_state.MOCK_RESPONSE_CHUNKS)
            generated_tokens = 1
            finish_reason = (
                core_pb2.FINISH_REASON_STOP
                if done
                else core_pb2.FINISH_REASON_UNSPECIFIED
            )

            if done:
                clear_request(item.request_id)
            else:
                request_state.set_decode_index(item.request_id, word_index)

        return execute_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=done,
            token_id=random.randint(1, 200),
            finish_reason=finish_reason,
            computed_tokens=0,
            generated_tokens=generated_tokens,
            execution_ms=latency_ms,
            error_message="",
        )

def clear_request(request_id: str) -> None:
    request_state.clear_decode_index(request_id)
    block.clear_runtime(request_id)


def prefill_latency_ms(scheduled_tokens: int, per_token_ms: int, fixed_latency_ms: int) -> int:
    return scheduled_tokens * per_token_ms + fixed_latency_ms

def decode_latency_ms(block_table_size: int, new_blocks: int, jitter_ms: int) -> int:
    estimated = jitter_ms + block_table_size // 32 + new_blocks * 2
    return max(70, min(130, estimated))