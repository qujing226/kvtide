import unittest

import torch
from adapter.dynamic_cache import DynamicCacheAdapter
from kvtide.v1 import block_pb2, core_pb2, executor_pb2
from runner.transformers import Runner
from runtime.batch import BatchBuilder
from runtime.kv_cache import PagedKVCache
from transformers import Qwen3Config, Qwen3ForCausalLM


def build_tiny_runner(*, num_layers: int = 1, num_blocks: int = 2) -> Runner:
    config = Qwen3Config(
        vocab_size=32,
        hidden_size=16,
        intermediate_size=32,
        num_hidden_layers=num_layers,
        num_attention_heads=2,
        num_key_value_heads=1,
        head_dim=8,
        max_position_embeddings=32,
        eos_token_id=31,
    )
    runner = object.__new__(Runner)
    from runner.device import create_execution_timer

    runner.device = torch.device("cpu")
    runner.timer = create_execution_timer(runner.device)

    runner.model = Qwen3ForCausalLM(config).eval()
    runner.block_size = 4
    runner.num_layers = config.num_hidden_layers
    num_kv_heads = config.num_key_value_heads
    assert num_kv_heads is not None
    runner.num_kv_heads = num_kv_heads
    runner.head_dim = config.head_dim
    runner.batch_builder = BatchBuilder(runner.block_size)
    runner.kv_cache = PagedKVCache(
        num_layers=runner.num_layers,
        num_blocks=num_blocks,
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


class RunnerTest(unittest.IsolatedAsyncioTestCase):
    async def test_imported_prefix_matches_full_recompute_logits(self):
        with torch.random.fork_rng(devices=[]):
            torch.manual_seed(0)
            source = build_tiny_runner(num_layers=2, num_blocks=4)
            destination = build_tiny_runner(num_layers=2, num_blocks=4)
        destination.model.load_state_dict(source.model.state_dict())

        prefix = [1, 5, 2, 9, 3, 7, 4, 11]
        suffix = [6, 10]
        prefill = execute_item(
            work_id="source-prefill",
            phase=core_pb2.WORK_PHASE_PREFILL,
            token_ids=prefix,
            computed_tokens=0,
        )
        # Logical prefix order deliberately differs from physical block order.
        prefill.kv_blocks.block_table[:] = [2, 0]
        prefill.kv_blocks.allocated_blocks.extend([2, 0])
        await source.execute([prefill])

        key, value = source.kv_cache.export_blocks([2, 0])
        destination.kv_cache.import_blocks([1, 3], key, value)

        # Attention over an entirely visible prefix can be invariant to paired
        # K/V permutations, so logits alone do not prove logical block order.
        for layer in range(source.num_layers):
            expected_kv = source.kv_cache.gather(layer, [2, 0], len(prefix))
            imported_kv = destination.kv_cache.gather(layer, [1, 3], len(prefix))
            for actual, expected in zip(imported_kv, expected_kv):
                torch.testing.assert_close(actual, expected, rtol=0, atol=0)

        # The destination must own its data, independently of source lifetime
        # and of the temporary transfer tensors.
        await source.release_blocks([2, 0])
        source.kv_cache.key_cache.zero_()
        source.kv_cache.value_cache.zero_()
        key.zero_()
        value.zero_()

        continuation = execute_item(
            work_id="destination-suffix",
            phase=core_pb2.WORK_PHASE_PREFILL,
            token_ids=suffix,
            computed_tokens=len(prefix),
        )
        continuation.kv_blocks.block_table[:] = [1, 3, 0]
        # Imported prefix blocks are already ready; only the suffix block is new.
        continuation.kv_blocks.allocated_blocks.append(0)

        captured_logits: list[torch.Tensor] = []

        def capture_logits(_module, _inputs, output):
            # Observe the real Runner forward without changing its output.
            captured_logits.append(output.logits[:, -1, :].detach().clone())

        hook = destination.model.register_forward_hook(capture_logits)
        try:
            results = await destination.execute([continuation])
        finally:
            hook.remove()

        # Independent reference: no paged cache, adapter, or imported KV.
        full_tokens = torch.tensor([prefix + suffix], dtype=torch.long)
        with torch.inference_mode():
            reference = source.model(
                input_ids=full_tokens,
                position_ids=torch.arange(full_tokens.shape[1]).unsqueeze(0),
                use_cache=False,
                logits_to_keep=1,
            ).logits[:, -1, :]

        self.assertEqual(len(captured_logits), 1)
        # FP32 cached and full forwards can differ slightly in operation order.
        torch.testing.assert_close(
            captured_logits[0], reference, rtol=1e-4, atol=1e-5
        )
        self.assertEqual(results[0].token_id, int(reference.argmax(dim=-1).item()))
        self.assertTrue(
            bool(destination.kv_cache.valid_slots[:, [1, 3], :].all().item())
        )
        self.assertTrue(
            bool(destination.kv_cache.valid_slots[:, 0, : len(suffix)].all().item())
        )
        self.assertFalse(bool(source.kv_cache.valid_slots.any().item()))

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

    async def test_execute_invalidates_newly_allocated_block_before_forward(self):
        runner = build_tiny_runner()

        # 模拟 block 0 残留了上一个请求的数据。
        runner.kv_cache.valid_slots[:, 0, :] = True

        item = execute_item(
            work_id="prefill",
            phase=core_pb2.WORK_PHASE_PREFILL,
            token_ids=[1, 2],
            computed_tokens=0,
        )
        item.kv_blocks.allocated_blocks.append(0)

        await runner.execute([item])

        # 当前 forward 重写了 slot 0、1。
        self.assertTrue(bool(runner.kv_cache.valid_slots[:, 0, :2].all().item()))
        # 上个请求留下的 slot 2、3 必须失效。
        self.assertFalse(bool(runner.kv_cache.valid_slots[:, 0, 2:].any().item()))


if __name__ == "__main__":
    unittest.main()
