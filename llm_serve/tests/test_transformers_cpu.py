import unittest

import torch
from transformers import Qwen3Config, Qwen3ForCausalLM

from adapter.dynamic_cache import DynamicCacheAdapter
from mini_llm_serve.v1 import block_pb2, core_pb2, executor_pb2
from runner.transformers_cpu import QwenTransformersRunner
from runtime.batch import BatchBuilder
from runtime.kv_cache import PagedKVCache


def build_tiny_runner() -> QwenTransformersRunner:
    config = Qwen3Config(
        vocab_size=32,
        hidden_size=16,
        intermediate_size=32,
        num_hidden_layers=1,
        num_attention_heads=2,
        num_key_value_heads=1,
        head_dim=8,
        max_position_embeddings=32,
        eos_token_id=31,
    )
    runner = object.__new__(QwenTransformersRunner)
    runner.model = Qwen3ForCausalLM(config).eval()
    runner.block_size = 4
    runner.num_layers = config.num_hidden_layers
    runner.num_kv_heads = config.num_key_value_heads
    runner.head_dim = config.head_dim
    runner.batch_builder = BatchBuilder(runner.block_size)
    runner.kv_cache = PagedKVCache(
        num_layers=runner.num_layers,
        num_blocks=2,
        block_size=runner.block_size,
        num_kv_heads=runner.num_kv_heads,
        head_dim=runner.head_dim,
        dtype=torch.float32,
        device="cpu",
    )
    runner.cache_adapter = DynamicCacheAdapter(runner.kv_cache, config)
    runner.eos_token_ids = set()
    return runner


def execute_item(
    *,
    work_id: str,
    phase: core_pb2.WorkPhase,
    token_ids: list[int],
    computed_tokens: int,
) -> executor_pb2.ExecuteItem:
    return executor_pb2.ExecuteItem(
        work_id=work_id,
        request_id="request",
        phase=phase,
        token_ids=token_ids,
        computed_tokens=computed_tokens,
        num_new_tokens=len(token_ids),
        kv_blocks=block_pb2.KVBlockMetadata(
            block_size=4,
            block_table=[0],
        ),
        sample=True,
    )


class QwenTransformersRunnerTest(unittest.IsolatedAsyncioTestCase):
    async def test_prefill_then_decode_reuses_and_extends_paged_kv_cache(self):
        torch.manual_seed(0)
        runner = build_tiny_runner()

        prefill = await runner.execute(
            [
                execute_item(
                    work_id="prefill",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                    token_ids=[1, 2],
                    computed_tokens=0,
                )
            ]
        )

        self.assertEqual(prefill[0].computed_tokens, 2)
        self.assertEqual(prefill[0].generated_tokens, 1)
        self.assertTrue(bool(runner.kv_cache.valid_slots[:, 0, :2].all().item()))

        decode = await runner.execute(
            [
                execute_item(
                    work_id="decode",
                    phase=core_pb2.WORK_PHASE_DECODE,
                    token_ids=[prefill[0].token_id],
                    computed_tokens=2,
                )
            ]
        )

        self.assertEqual(decode[0].computed_tokens, 0)
        self.assertEqual(decode[0].generated_tokens, 1)
        self.assertTrue(bool(runner.kv_cache.valid_slots[:, 0, :3].all().item()))

        for layer_idx in range(runner.num_layers):
            key, value = runner.kv_cache.gather(
                layer_idx=layer_idx,
                block_table=[0],
                context_len=3,
            )
            self.assertEqual(tuple(key.shape), (1, 3, 8))
            self.assertEqual(tuple(value.shape), (1, 3, 8))


if __name__ == "__main__":
    unittest.main()
