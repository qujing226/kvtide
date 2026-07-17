import torch
from transformers import DynamicCache, PreTrainedConfig

from runtime.kv_cache import PagedKVCache


class DynamicCacheAdapter:
    def __init__(
        self,
        paged_cache: PagedKVCache,
        model_config: PreTrainedConfig | None = None,
    ):
        self.paged_cache = paged_cache
        self.model_config = model_config

    def build(
        self,
        block_table: list[int],
        past_len: int,
    ) -> DynamicCache:
        if past_len < 0:
            raise ValueError("past_len must not be negative")

        layer_data: list[tuple[torch.Tensor, torch.Tensor]] = []

        for layer_idx in range(self.paged_cache.num_layers):
            key, value = self.paged_cache.gather(
                layer_idx=layer_idx,
                block_table=block_table,
                context_len=past_len,
            )

            # Paged: [H, P, D] -> HD: [1, H, P, D]
            layer_data.append(
                (
                    key.unsqueeze(0),
                    value.unsqueeze(0),
                )
            )
        return DynamicCache(
            ddp_cache_data=layer_data,
            config=self.model_config,
        )

    def write_new(
        self,
        cache: DynamicCache,
        slot_mapping: list[int],
    ) -> None:
        num_new_tokens = len(slot_mapping)
        if num_new_tokens == 0:
            return
        layers = list(cache)
        if len(layers) != self.paged_cache.num_layers:
            raise ValueError("DynamiCache layer count does not match PagedKVCache")

        for layer_idx, (key, value, _) in enumerate(layers):
            if not isinstance(key, torch.Tensor):
                raise ValueError(f"DynamicCache layer {layer_idx} has no key tensor")
            if not isinstance(value, torch.Tensor):
                raise ValueError(f"DynamicCache layer {layer_idx} has no value tensor")
            self._validate_hf_cache_tensor(key, num_new_tokens)
            self._validate_hf_cache_tensor(value, num_new_tokens)

            # HF: [1, H, P+T, D]
            # onlt take the last T token, then cut off batch dimension
            new_key = key[:, :, -num_new_tokens:, :].squeeze(0)
            new_value = value[:, :, -num_new_tokens:, :].squeeze(0)

            self.paged_cache.write(
                layer_idx=layer_idx,
                slot_mapping=slot_mapping,
                key=new_key,
                value=new_value,
            )

    def _validate_hf_cache_tensor(
        self,
        tensor: torch.Tensor,
        num_new_tokens: int,
    ) -> None:
        if tensor.ndim != 4:
            raise ValueError("HF cache tensor must have four dimensions")
        if tensor.shape[0] != 1:
            raise ValueError("only batch_size = 1 is currently supported")
        if tensor.shape[1] != self.paged_cache.num_kv_heads:
            raise ValueError("kv head count mismatch")
        if tensor.shape[2] < num_new_tokens:
            raise ValueError("HF cache contains fewer tokens than expected")
        if tensor.shape[3] != self.paged_cache.head_dim:
            raise ValueError("HEAD dimension mismatch")
