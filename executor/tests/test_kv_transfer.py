import unittest
import torch

from runtime.kv_cache import PagedKVCache
from runtime.kv_transfer import KVTransferRuntime, encode_tensor


def build_transfer(*, dtype=torch.float32, num_blocks=4, model=None, config=None):
    if model is None:
        model = torch.nn.Linear(2, 2, bias=False)
        with torch.no_grad():
            model.weight.fill_(0.25)
    cache = PagedKVCache(2, num_blocks, 4, 2, 2, dtype=dtype)
    cache.key_cache.zero_()
    cache.value_cache.zero_()
    return KVTransferRuntime(
        model=model,
        model_config=config or {"model_type": "test", "rope_theta": 10000},
        cache=cache,
    )


class KVTransferTest(unittest.TestCase):
    def test_compatibility_ignores_location_and_capacity_but_checks_weights_and_rope(self):
        source = build_transfer(config={"rope_theta": 10000, "_name_or_path": "/a"})
        destination = build_transfer(
            num_blocks=8, config={"_name_or_path": "/b", "rope_theta": 10000}
        )
        self.assertEqual(source.compatibility, destination.compatibility)
        self.assertNotEqual(
            source.compatibility.kv_compatibility_id,
            build_transfer(config={"rope_theta": 20000}).compatibility.kv_compatibility_id,
        )
        model = torch.nn.Linear(2, 2, bias=False)
        with torch.no_grad():
            model.weight.fill_(0.5)
        self.assertNotEqual(
            source.compatibility.model_revision,
            build_transfer(model=model).compatibility.model_revision,
        )
        self.assertNotEqual(
            source.compatibility.kv_compatibility_id,
            build_transfer(dtype=torch.bfloat16).compatibility.kv_compatibility_id,
        )

    def test_raw_bytes_round_trip_remaps_all_layers_including_bfloat16(self):
        for dtype in (torch.float32, torch.float16, torch.bfloat16):
            with self.subTest(dtype=dtype):
                source = build_transfer(dtype=dtype)
                destination = build_transfer(dtype=dtype)
                source.cache.key_cache.copy_(
                    torch.arange(source.cache.key_cache.numel()).reshape(source.cache.key_cache.shape)
                )
                source.cache.value_cache.copy_(source.cache.key_cache + 100)
                source.cache.valid_slots.fill_(True)
                key, value = source.export_blocks([2, 0])
                self.assertEqual(len(key), destination.payload_bytes(2))
                destination.import_blocks([1, 3], key, value)
                for layer in range(2):
                    expected = source.cache.gather(layer, [2, 0], 8)
                    actual = destination.cache.gather(layer, [1, 3], 8)
                    for a, b in zip(actual, expected):
                        torch.testing.assert_close(a, b, rtol=0, atol=0)

    def test_wire_encoding_is_little_endian(self):
        self.assertEqual(encode_tensor(torch.tensor([1.0])), b"\x00\x00\x80\x3f")
        self.assertEqual(
            encode_tensor(torch.tensor([1.0], dtype=torch.bfloat16)), b"\x80\x3f"
        )

    def test_bad_second_payload_does_not_write_first_payload(self):
        destination = build_transfer()
        key = encode_tensor(torch.ones(2, 2, 1, 4, 2))
        for value in (b"", key[:-1], key + b"x"):
            with self.subTest(length=len(value)):
                with self.assertRaises(ValueError):
                    destination.import_blocks([1], key, value)
                self.assertFalse(destination.cache.key_cache.any().item())
                self.assertFalse(destination.cache.valid_slots.any().item())
