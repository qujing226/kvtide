import unittest

import torch

from adapter.dynamic_cache import DynamicCacheAdapter
from runner.kv_cache import PagedKVCache


class DynamicCacheAdapterTest(unittest.TestCase):
    def test_round_trip_between_paged_and_dynamic_cache(self):
        paged_cache = PagedKVCache(
            num_layers=2,
            num_blocks=3,
            block_size=4,
            num_kv_heads=2,
            head_dim=2,
        )
        adapter = DynamicCacheAdapter(paged_cache)

        block_table = [2]
        history_slots = [8, 9]
        history_by_layer = []

        for layer_idx in range(2):
            key = (
                torch.arange(8, dtype=torch.float32)
                .view(2, 2, 2)
                .add(layer_idx * 100)
            )
            value = key + 1000
            history_by_layer.append((key, value))

            paged_cache.write(
                layer_idx=layer_idx,
                slot_mapping=history_slots,
                key=key,
                value=value,
            )

        dynamic_cache = adapter.build(
            block_table=block_table,
            past_len=2,
        )

        self.assertEqual(dynamic_cache.get_seq_length(), 2)

        for layer_idx, (cached_key, cached_value, _) in enumerate(dynamic_cache):
            expected_key, expected_value = history_by_layer[layer_idx]

            torch.testing.assert_close(
                cached_key,
                expected_key.unsqueeze(0),
            )
            torch.testing.assert_close(
                cached_value,
                expected_value.unsqueeze(0),
            )

        new_slots = [10]
        new_by_layer = []

        for layer_idx in range(2):
            new_key = torch.full(
                (1, 2, 1, 2),
                fill_value=200 + layer_idx,
            )
            new_value = new_key + 1000
            new_by_layer.append((new_key, new_value))

            dynamic_cache.update(
                new_key,
                new_value,
                layer_idx,
            )

        adapter.write_new(
            cache=dynamic_cache,
            slot_mapping=new_slots,
        )

        for layer_idx in range(2):
            gathered_key, gathered_value = paged_cache.gather(
                layer_idx=layer_idx,
                block_table=block_table,
                context_len=3,
            )
            history_key, history_value = history_by_layer[layer_idx]
            new_key, new_value = new_by_layer[layer_idx]

            torch.testing.assert_close(
                gathered_key,
                torch.cat(
                    [history_key, new_key.squeeze(0)],
                    dim=1,
                ),
            )
            torch.testing.assert_close(
                gathered_value,
                torch.cat(
                    [history_value, new_value.squeeze(0)],
                    dim=1,
                ),
            )


if __name__ == "__main__":
    unittest.main()