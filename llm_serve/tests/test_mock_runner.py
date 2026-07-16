import unittest

from mini_llm_serve.v1 import block_pb2, core_pb2, executor_pb2
from runner.mock import MockRunner


def execute_item(
    work_id: str,
    phase: core_pb2.WorkPhase,
) -> executor_pb2.ExecuteItem:
    return executor_pb2.ExecuteItem(
        work_id=work_id,
        request_id=f"request-{work_id}",
        phase=phase,
        token_ids=[1],
        num_new_tokens=1,
        kv_blocks=block_pb2.KVBlockMetadata(
            block_size=16,
            block_table=[0],
        ),
    )


class RecordingMockRunner(MockRunner):
    async def prefill_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        return executor_pb2.ExecuteResult(work_id=item.work_id)

    async def decode_one(
        self, item: executor_pb2.ExecuteItem
    ) -> executor_pb2.ExecuteResult:
        return executor_pb2.ExecuteResult(work_id=item.work_id)


class MockRunnerTest(unittest.IsolatedAsyncioTestCase):
    async def test_execute_preserves_mixed_batch_order(self):
        runner = RecordingMockRunner()
        items = [
            execute_item("p1", core_pb2.WORK_PHASE_PREFILL),
            execute_item("d1", core_pb2.WORK_PHASE_DECODE),
            execute_item("p2", core_pb2.WORK_PHASE_PREFILL),
        ]

        results = await runner.execute(items)

        self.assertEqual([result.work_id for result in results], ["p1", "d1", "p2"])

    def test_runtime_state_is_owned_by_each_runner(self):
        first = MockRunner()
        second = MockRunner()

        first._decode_positions["request"] = 3

        self.assertNotIn("request", second._decode_positions)
        self.assertIsNot(first._kv_runtime, second._kv_runtime)


if __name__ == "__main__":
    unittest.main()
