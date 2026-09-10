"""Layout-v1 KV identity and blocking host-staged transport primitives.

No pickle, RPC, allocation policy, or automatic dtype conversion on the wire.
The model and its effective configuration must remain immutable after startup.
"""

import hashlib
import json
import sys
from dataclasses import dataclass
from importlib.metadata import version

import torch

from runtime.kv_cache import PagedKVCache

KV_LAYOUT_VERSION = 1
_WIRE_DTYPES = {torch.float32, torch.float16, torch.bfloat16}


def _canonical_json(value: object) -> bytes:
    return json.dumps(
        value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False
    ).encode("utf-8")


def _tensor_bytes(tensor: torch.Tensor) -> memoryview:
    if sys.byteorder != "little":
        raise ValueError("KV layout v1 currently requires a little-endian host")
    raw = tensor.detach().cpu().contiguous().reshape(-1).view(torch.uint8)
    return memoryview(raw.numpy()).cast("B")


def encode_tensor(tensor: torch.Tensor) -> bytes:
    """Preserve FP32/FP16/BF16 bit patterns, including non-contiguous inputs."""
    if tensor.dtype not in _WIRE_DTYPES:
        raise ValueError("unsupported KV wire dtype")
    return bytes(_tensor_bytes(tensor))


def _model_revision(model: torch.nn.Module) -> str:
    # Hash actual loaded weights, not a mutable repository name or local path.
    # Only one parameter/buffer is staged on the CPU at a time.
    digest = hashlib.sha256(b"kvtide-loaded-weights-v1\0")
    for name, tensor in sorted(model.state_dict().items()):
        header = _canonical_json([name, str(tensor.dtype), list(tensor.shape)])
        digest.update(len(header).to_bytes(8, "big"))
        digest.update(header)
        digest.update(_tensor_bytes(tensor))
    return "sha256:" + digest.hexdigest()


@dataclass(frozen=True, slots=True)
class KVCompatibility:
    model_revision: str
    kv_layout_version: int
    kv_compatibility_id: str


class KVTransferRuntime:
    def __init__(
        self,
        *,
        model: torch.nn.Module,
        model_config: dict,
        cache: PagedKVCache,
        tensor_parallel_size: int = 1,
    ):
        if tensor_parallel_size != 1:
            raise ValueError("KV transfer v1 supports tensor_parallel_size=1 only")
        if cache.key_cache.dtype not in _WIRE_DTYPES:
            raise ValueError("unsupported KV wire dtype")
        self.cache = cache
        config = {
            key: value
            for key, value in model_config.items()
            if key not in {"_name_or_path", "_commit_hash", "transformers_version"}
        }
        revision = _model_revision(model)
        descriptor = {
            "version": 1,
            "model_revision": revision,
            "model_class": f"{type(model).__module__}.{type(model).__qualname__}",
            "model_config": config,
            "torch_version": str(torch.__version__),
            "transformers_version": version("transformers"),
            "attention_implementation": getattr(
                getattr(model, "config", None), "_attn_implementation", None
            ),
            "dtype": str(cache.key_cache.dtype),
            "num_layers": cache.num_layers,
            "num_kv_heads": cache.num_kv_heads,
            "head_dim": cache.head_dim,
            "block_size": cache.block_size,
            "tensor_parallel_size": tensor_parallel_size,
            "layout_version": KV_LAYOUT_VERSION,
            "positions": "complete-prefix-from-zero",
        }
        self.compatibility = KVCompatibility(
            revision,
            KV_LAYOUT_VERSION,
            hashlib.sha256(_canonical_json(descriptor)).hexdigest(),
        )

    def payload_bytes(self, num_blocks: int) -> int:
        """Expected byte length of K alone (and of V alone), no valid flags."""
        if num_blocks <= 0:
            raise ValueError("a transfer must contain at least one block")
        c = self.cache
        return (
            c.num_layers
            * c.num_kv_heads
            * num_blocks
            * c.block_size
            * c.head_dim
            * c.key_cache.element_size()
        )

    def validate_destination(self, block_ids: list[int]) -> None:
        c = self.cache
        if not block_ids or len(set(block_ids)) != len(block_ids):
            raise ValueError("destination block IDs must be nonempty and unique")
        if any(not isinstance(i, int) or i < 0 or i >= c.num_blocks for i in block_ids):
            raise ValueError("invalid destination block ID")
        if bool(c.valid_slots[:, block_ids, :].any().item()):
            raise ValueError("destination blocks contain valid KV")

    def export_blocks(self, block_ids: list[int]) -> tuple[bytes, bytes]:
        key, value = self.cache.export_blocks(block_ids)
        return encode_tensor(key), encode_tensor(value)

    def import_blocks(self, block_ids: list[int], key: bytes, value: bytes) -> None:
        self.validate_destination(block_ids)
        expected = self.payload_bytes(len(block_ids))
        if len(key) != expected or len(value) != expected:
            raise ValueError(f"each KV payload must have exactly {expected} bytes")
        if sys.byteorder != "little":
            raise ValueError("KV layout v1 currently requires a little-endian host")
        c = self.cache
        shape = (c.num_layers, c.num_kv_heads, len(block_ids), c.block_size, c.head_dim)
        # Own writable buffers; torch.frombuffer must not alias immutable bytes.
        key_tensor = torch.frombuffer(bytearray(key), dtype=c.key_cache.dtype).reshape(
            shape
        )
        value_tensor = torch.frombuffer(
            bytearray(value), dtype=c.value_cache.dtype
        ).reshape(shape)
        # Stage BOTH payloads before changing any destination slots.
        key_tensor = key_tensor.to(c.key_cache.device)
        value_tensor = value_tensor.to(c.value_cache.device)
        try:
            c.import_blocks(block_ids, key_tensor, value_tensor)
            if c.key_cache.device.type == "cuda":
                # v1 is deliberately blocking. The service lock prevents readers
                # from observing valid flags before GPU completion is confirmed.
                torch.cuda.synchronize(c.key_cache.device)
        except Exception:
            self.invalidate_blocks(block_ids)
            raise

    def invalidate_blocks(self, block_ids: list[int]) -> None:
        self.cache.release(block_ids)
        if self.cache.key_cache.device.type == "cuda":
            torch.cuda.synchronize(self.cache.key_cache.device)
