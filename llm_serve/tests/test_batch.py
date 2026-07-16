import unittest

from mini_llm_serve.v1 import block_pb2, core_pb2, executor_pb2
from runtime.batch import BatchBuilder


def execute_item(
    *,
    work_id: str,
    phase: core_pb2.WorkPhase,
    token_ids: list[int],
    computed_tokens: int,
    block_table: list[int],
    sample: bool = False,
    block_size: int = 16,
    num_new_tokens: int | None = None,
) -> executor_pb2.ExecuteItem:
    return executor_pb2.ExecuteItem(
        work_id=work_id,
        request_id=f"request-{work_id}",
        phase=phase,
        token_ids=token_ids,
        computed_tokens=computed_tokens,
        num_new_tokens=(len(token_ids) if num_new_tokens is None else num_new_tokens),
        kv_blocks=block_pb2.KVBlockMetadata(
            block_size=block_size,
            block_table=block_table,
        ),
        sample=sample,
    )


class BatchBuilderTest(unittest.TestCase):
    def test_builds_flattened_metadata_for_mixed_batch(self):
        items = [
            execute_item(
                work_id="decode",
                phase=core_pb2.WORK_PHASE_DECODE,
                token_ids=[90],
                computed_tokens=17,
                block_table=[4, 9],
                sample=True,
            ),
            execute_item(
                work_id="prefill",
                phase=core_pb2.WORK_PHASE_PREFILL,
                token_ids=[21, 22, 23],
                computed_tokens=16,
                block_table=[7, 3],
                sample=True,
            ),
        ]

        batch = BatchBuilder(block_size=16).build(items)

        self.assertEqual(batch.items, items)
        self.assertEqual(batch.input_ids, [90, 21, 22, 23])
        self.assertEqual(batch.positions, [17, 16, 17, 18])
        self.assertEqual(batch.query_start_locs, [0, 1, 4])
        self.assertEqual(batch.context_lens, [18, 19])
        self.assertEqual(batch.slot_mapping, [145, 48, 49, 50])
        self.assertEqual(batch.block_tables, [[4, 9], [7, 3]])
        self.assertEqual(batch.sample_indices, [0, 3])
        self.assertEqual(batch.sample_item_indices, [0, 1])

    def test_pads_block_tables_to_equal_length(self):
        items = [
            execute_item(
                work_id="short",
                phase=core_pb2.WORK_PHASE_PREFILL,
                token_ids=[5],
                computed_tokens=0,
                block_table=[1],
            ),
            execute_item(
                work_id="long",
                phase=core_pb2.WORK_PHASE_DECODE,
                token_ids=[6],
                computed_tokens=16,
                block_table=[2, 3],
                sample=True,
            ),
        ]

        batch = BatchBuilder(block_size=16).build(items)

        self.assertEqual(batch.block_tables, [[1, -1], [2, 3]])
        self.assertEqual(batch.slot_mapping, [16, 48])

    def test_rejects_invalid_item_metadata(self):
        invalid_items = [
            (
                "empty tokens",
                execute_item(
                    work_id="empty",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                    token_ids=[],
                    computed_tokens=0,
                    block_table=[],
                ),
                "must contain tokens",
            ),
            (
                "new token count",
                execute_item(
                    work_id="count",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                    token_ids=[1],
                    computed_tokens=0,
                    block_table=[0],
                    num_new_tokens=2,
                ),
                "token count",
            ),
            (
                "block size",
                execute_item(
                    work_id="size",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                    token_ids=[1],
                    computed_tokens=0,
                    block_table=[0],
                    block_size=8,
                ),
                "block size mismatch",
            ),
            (
                "capacity",
                execute_item(
                    work_id="capacity",
                    phase=core_pb2.WORK_PHASE_DECODE,
                    token_ids=[1],
                    computed_tokens=16,
                    block_table=[0],
                ),
                "capacity is insufficient",
            ),
        ]

        builder = BatchBuilder(block_size=16)
        for name, item, message in invalid_items:
            with self.subTest(name=name):
                with self.assertRaisesRegex(ValueError, message):
                    builder.build([item])


if __name__ == "__main__":
    unittest.main()
