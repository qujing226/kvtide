import torch


class PagedKVCache:
    def __init__(
        self,
        num_layers: int,
        num_blocks: int,
        block_size: int,
        num_kv_heads: int,
        head_dim: int,
        dtype: torch.dtype = torch.float32,
        device: str | torch.device = "cpu",
    ):
        dimensions = (num_layers, num_blocks, block_size, num_kv_heads, head_dim)
        if any(value <= 0 for value in dimensions):
            raise ValueError("KV cache dimensions must be positive")

        self.num_layers = num_layers
        self.num_blocks = num_blocks
        self.block_size = block_size
        self.num_kv_heads = num_kv_heads
        self.head_dim = head_dim

        # [layer, physical block, block offset, KV head, head dimension]
        shape = (
            num_layers,
            num_blocks,
            block_size,
            num_kv_heads,
            head_dim,
        )

        self.key_cache = torch.empty(shape, dtype=dtype, device=device)
        self.value_cache = torch.empty(shape, dtype=dtype, device=device)

        # each layer records whether the correspnding slot has been written
        self.valid_slots = torch.zeros(
            (num_layers, num_blocks, block_size),
            dtype=torch.bool,
            device=device,
        )

    def write(
        self,
        layer_idx: int,
        slot_mapping: list[int],
        key: torch.Tensor,
        value: torch.Tensor,
    ) -> None:
        self._validate_layer(layer_idx)

        slots = torch.tensor(
            slot_mapping,
            dtype=torch.long,
            device=self.key_cache.device,
        )
        self._validate_slots(slots)

        expected_shape = (
            len(slot_mapping),
            self.num_kv_heads,
            self.head_dim,
        )
        if tuple(key.shape) != expected_shape:
            raise ValueError(f"invalid key shape: expected {expected_shape}")
        if tuple(value.shape) != expected_shape:
            raise ValueError(f"invalid value shape: expected {expected_shape}")

        key = key.to(device=self.key_cache.device, dtype=self.key_cache.dtype)
        value = value.to(device=self.value_cache.device, dtype=self.value_cache.dtype)

        # platten [block, offset] into slot
        flat_key = self.key_cache[layer_idx].view(-1, self.num_kv_heads, self.head_dim)
        flat_value = self.value_cache[layer_idx].view(
            -1, self.num_kv_heads, self.head_dim
        )
        flat_valid = self.valid_slots[layer_idx].view(-1)

        flat_key.index_copy_(0, slots, key)
        flat_value.index_copy_(0, slots, value)
        flat_valid[slots] = True

    def gather(
        self,
        layer_idx: int,
        block_table: list[int],
        context_len: int,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        self._validate_layer(layer_idx)

        if context_len < 0:
            raise ValueError("context_len must not ve negatinve")

        if context_len == 0:
            shape = (0, self.num_kv_heads, self.head_dim)
            return (
                torch.empty(
                    shape,
                    dtype=self.key_cache.dtype,
                    device=self.key_cache.device,
                ),
                torch.empty(
                    shape,
                    dtype=self.value_cache.dtype,
                    device=self.value_cache.device,
                ),
            )

        required_blocks = (context_len + self.block_size - 1) // self.block_size
        if len(block_table) < required_blocks:
            raise ValueError("block table is too short")

        table = torch.tensor(
            block_table[:required_blocks],
            dtype=torch.long,
            device=self.key_cache.device,
        )
        if torch.any(table < 0) or torch.any(table >= self.num_blocks):
            raise ValueError("block table contains invalid block id")

        positions = torch.arange(
            context_len,
            dtype=torch.long,
            device=self.key_cache.device,
        )

        logical_blocks = positions // self.block_size
        block_offsets = positions % self.block_size
        physical_blocks = table[logical_blocks]
        slots = physical_blocks * self.block_size + block_offsets

        flat_valid = self.valid_slots[layer_idx].view(-1)
        if not bool(flat_valid.index_select(0, slots).all().item()):
            raise ValueError("requested KV slot has not been written")

        flat_key = self.key_cache[layer_idx].view(-1, self.num_kv_heads, self.head_dim)
        flat_value = self.value_cache[layer_idx].view(
            -1, self.num_kv_heads, self.head_dim
        )

        return (
            flat_key.index_select(0, slots),
            flat_value.index_select(0, slots),
        )

    def release(self, block_ids: list[int]) -> None:
        if not block_ids:
            return

        ids = torch.tensor(
            block_ids,
            dtype=torch.long,
            device=self.valid_slots.device,
        )
        if torch.any(ids < 0) or torch.any(ids >= self.num_blocks):
            raise ValueError("invalid block id")

        # soft delete
        self.valid_slots[:, ids, :] = False

    @property
    def cache_bytes(self) -> int:
        elements = self.key_cache.numel() + self.value_cache.numel()
        return elements * self.key_cache.element_size()

    def _validate_layer(self, layer_idx: int) -> None:
        if not 0 <= layer_idx < self.num_layers:
            raise ValueError("invalid layer index")

    def _validate_slots(self, slots: torch.Tensor) -> None:
        capacity = self.num_blocks * self.block_size
        if torch.any(slots < 0) or torch.any(slots >= capacity):
            raise ValueError("slot mapping contains invalid slot")
        if slots.unique().numel() != slots.numel():
            raise ValueError("slot mapping contains duplicate slots")
