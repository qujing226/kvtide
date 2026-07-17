import asyncio
import random
import re
from dataclasses import dataclass, field

from kvtide.v1 import core_pb2, executor_pb2
from runner.base import ModelRunner, RuntimeInfo


MOCK_RESPONSE_TEXT = """# Paged Attention

vLLM uses a multi-head attention kernel that works with paged KV caches. Instead of reserving one contiguous region for every request, the cache is divided into fixed-size blocks managed by the serving system.

## Why blocks help

- Requests can grow without relocating an entire KV cache.
- Freed blocks can return to a shared pool.
- Prefix blocks can be reused when requests share the same cached prompt.

During **prefill**, the executor computes the prompt and writes its key/value states into these blocks. During **decode**, each generated token reads the existing block table and appends new state when required.

This block is a serving-memory concept. It is different from a CUDA thread block, which describes how GPU threads are grouped for kernel execution."""
MOCK_RESPONSE_CHUNKS = re.findall(r"\S+\s*", MOCK_RESPONSE_TEXT)


@dataclass(slots=True)
class KVRuntimeState:
    block_size: int = 0
    block_table: list[int] = field(default_factory=list)
    computed_tokens: int = 0


class MockRunner(ModelRunner):
    def __init__(self) -> None:
        super().__init__()
        self._runtime_info = RuntimeInfo(
            model_type="mock",
            dtype="none",
            block_size=16,
            num_kv_blocks=1024,
            num_hidden_layers=0,
            num_kv_heads=0,
            head_dim=0,
            total_memory_bytes=0,
            available_memory_bytes=0,
            kv_cache_bytes=0,
        )
        self._kv_runtime: dict[str, KVRuntimeState] = {}
        self._decode_positions: dict[str, int] = {}

    async def execute(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        tasks = [
            self.prefill_one(item)
            if item.phase == core_pb2.WORK_PHASE_PREFILL
            else self.decode_one(item)
            for item in items
        ]
        return list(await asyncio.gather(*tasks))

    async def prefill_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        # Refresh the executor-side KV metadata shadow from the control plane.
        kv_state = self._update_runtime(item)

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
        self._kv_runtime[item.request_id] = kv_state

        return executor_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            token_id=random.randint(1, 200),
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=computed_delta,
            generated_tokens=0 if not item.sample else 1,
            execution_ms=latency_ms,
            error_message="",
        )

    async def decode_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        # Refresh the executor-side KV metadata shadow for this decode step.
        kv_state = self._update_runtime(item)

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
        word_index = max(self._decode_positions.get(item.request_id, 0), item.generated_tokens)

        if word_index >= len(MOCK_RESPONSE_CHUNKS):
            self._clear_request(item.request_id)
            done = True
            generated_tokens = 0
            finish_reason = core_pb2.FINISH_REASON_STOP
        else:
            word_index += 1
            done = word_index >= len(MOCK_RESPONSE_CHUNKS)
            generated_tokens = 1
            finish_reason = (
                core_pb2.FINISH_REASON_STOP
                if done
                else core_pb2.FINISH_REASON_UNSPECIFIED
            )

            if done:
                self._clear_request(item.request_id)
            else:
                self._decode_positions[item.request_id] = word_index

        return executor_pb2.ExecuteResult(
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

    async def release_blocks(self, block_ids: list[int]) -> None:
        return None

    @property
    def runtime_info(self) -> RuntimeInfo:
        return self._runtime_info

    def _update_runtime(self, item: executor_pb2.ExecuteItem) -> KVRuntimeState:
        state = self._kv_runtime.get(item.request_id, KVRuntimeState())
        state.block_size = item.kv_blocks.block_size
        state.block_table = list(item.kv_blocks.block_table)
        state.computed_tokens = max(state.computed_tokens, item.computed_tokens)
        self._kv_runtime[item.request_id] = state
        return state

    def _clear_request(self, request_id: str) -> None:
        self._decode_positions.pop(request_id, None)
        self._kv_runtime.pop(request_id, None)


def prefill_latency_ms(
    scheduled_tokens: int, per_token_ms: int, fixed_latency_ms: int
) -> int:
    return scheduled_tokens * per_token_ms + fixed_latency_ms


def decode_latency_ms(block_table_size: int, new_blocks: int, jitter_ms: int) -> int:
    estimated = jitter_ms + block_table_size // 32 + new_blocks * 2
    return max(70, min(130, estimated))
