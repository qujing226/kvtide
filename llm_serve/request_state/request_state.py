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

decode_positions: dict[str, int] = {}


def set_decode_index(request_id: str, index: int) -> None:
    decode_positions[request_id] = index


def get_decode_index(request_id: str) -> int:
    return decode_positions.get(request_id, 0)


def clear_decode_index(request_id: str) -> None:
    decode_positions.pop(request_id, None)
