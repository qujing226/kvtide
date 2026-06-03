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