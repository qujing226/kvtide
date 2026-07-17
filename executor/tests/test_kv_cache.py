import unittest

import torch

from runtime import PagedKVCache


class PagedKVCacheTest(unittest.TestCase):
    def setUp(self):
        self.cache = PagedKVCache(
            num_layers=2,
            num_blocks=3,
            block_size=4,
            num_kv_heads=1,
            head_dim=2,
        )

    def test_writes_physical_slots_and_gathers_in_logical_order(self):
        # Logical blocks [0, 1] are stored in physical blocks [2, 0].
        block_table = [2, 0]
        slot_mapping = [8, 9, 10, 11, 0, 1]
        key = torch.tensor(
            [
                [
                    [10.0, 11.0],
                    [20.0, 21.0],
                    [30.0, 31.0],
                    [40.0, 41.0],
                    [50.0, 51.0],
                    [60.0, 61.0],
                ]
            ]
        )
        value = key + 1000

        self.cache.write(
            layer_idx=0,
            slot_mapping=slot_mapping,
            key=key,
            value=value,
        )
        gathered_key, gathered_value = self.cache.gather(
            layer_idx=0,
            block_table=block_table,
            context_len=6,
        )

        torch.testing.assert_close(gathered_key, key)
        torch.testing.assert_close(gathered_value, value)

    def test_rejects_slots_not_written_for_the_requested_layer(self):
        self.cache.write(
            layer_idx=0,
            slot_mapping=[8],
            key=torch.tensor([[[1.0, 2.0]]]),
            value=torch.tensor([[[3.0, 4.0]]]),
        )

        with self.assertRaisesRegex(ValueError, "has not been written"):
            self.cache.gather(
                layer_idx=1,
                block_table=[2],
                context_len=1,
            )

    def test_release_invalidates_a_block_in_every_layer(self):
        for layer_idx in range(2):
            self.cache.write(
                layer_idx=layer_idx,
                slot_mapping=[8],
                key=torch.tensor([[[1.0, 2.0]]]),
                value=torch.tensor([[[3.0, 4.0]]]),
            )

        self.cache.release([2])

        for layer_idx in range(2):
            with self.subTest(layer_idx=layer_idx):
                with self.assertRaisesRegex(ValueError, "has not been written"):
                    self.cache.gather(
                        layer_idx=layer_idx,
                        block_table=[2],
                        context_len=1,
                    )


if __name__ == "__main__":
    unittest.main()
