from dataclasses import dataclass, field

from mini_llm_serve.v1 import execute_pb2


@dataclass
class KVRuntimeState:
    block_size: int = 0
    block_table: list[int] = field(default_factory=list)
    computed_tokens: int = 0


kv_runtime: dict[str, KVRuntimeState] = {}


def update_runtime(item: execute_pb2.ExecuteItem) -> KVRuntimeState:
    state = kv_runtime.get(item.request_id, KVRuntimeState())
    state.block_size = item.kv_blocks.block_size
    state.block_table = list(item.kv_blocks.block_table)
    state.computed_tokens = max(state.computed_tokens, item.computed_tokens)
    kv_runtime[item.request_id] = state
    return state


def set_runtime(request_id: str, kv_state: KVRuntimeState) -> None:
    kv_runtime[request_id] = kv_state


def clear_runtime(request_id: str) -> None:
    kv_runtime.pop(request_id, None)
