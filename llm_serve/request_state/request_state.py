import re


MOCK_RESPONSE_TEXT = """# Paged Attention

vLLM uses a multi-head attention kernel that works with paged KV caches. Instead of reserving one contiguous region for every request, the cache is divided into fixed-size blocks managed by the serving system.

## Why blocks help

- Requests can grow without relocating an entire KV cache.
- Freed blocks can return to a shared pool.
- Prefix blocks can be reused when requests share the same cached prompt.

During **prefill**, the executor computes the prompt and writes its key/value states into these blocks. During **decode**, each generated token reads the existing block table and appends new state when required.

This block is a serving-memory concept. It is different from a CUDA thread block, which describes how GPU threads are grouped for kernel execution."""

# Keep trailing whitespace in each chunk so streamed concatenation preserves
# Markdown paragraphs and list formatting exactly.
MOCK_RESPONSE_CHUNKS = re.findall(r"\S+\s*", MOCK_RESPONSE_TEXT)

decode_positions: dict[str, int] = {}


def set_decode_index(request_id: str, index: int) -> None:
    decode_positions[request_id] = index


def get_decode_index(request_id: str) -> int:
    return decode_positions.get(request_id, 0)


def clear_decode_index(request_id: str) -> None:
    decode_positions.pop(request_id, None)
