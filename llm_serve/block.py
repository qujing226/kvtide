from dataclasses import dataclass, field
from mini_llm_serve.v1 import execute_pb2


@dataclass
class KVRuntimeState:
    block_size: int = 0
    block_table: list[int] = field(default_factory=list)
    computed_tokens: int =0 

request_state: dict[str, int] = {}
kv_runtime: dict[str, KVRuntimeState] = {}

    
def _update_kv_runtime(item: execute_pb2.ExecuteItem) -> KVRuntimeState:
    state = kv_runtime.get(item.request_id, KVRuntimeState(
        block_size = item.kv_blocks.block_size,
        block_table =list(item.kv_blocks.block_table),
        computed_tokens = max(state.computed_tokens, item.computed_tokens)
    ))
    kv_runtime[item.request_id] = state
    return state