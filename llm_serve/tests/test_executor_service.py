import unittest
from typing import cast
from unittest.mock import Mock

from connectrpc.request import RequestContext
from mini_llm_serve.v1 import core_pb2, executor_pb2
from executor_service import ExecuteServiceImpl
from runner import ModelRunner, RuntimeInfo
from setting import ExecutorConfig, RunnerConfig, RuntimeConfig


def result(item: executor_pb2.ExecuteItem) -> executor_pb2.ExecuteResult:
    return executor_pb2.ExecuteResult(
        work_id=item.work_id,
        request_id=item.request_id,
    )


class RecordingRunner(ModelRunner):
    def __init__(self):
        self.execute_batches: list[list[executor_pb2.ExecuteItem]] = []
        self.prefill_batches: list[list[executor_pb2.ExecuteItem]] = []
        self.decode_batches: list[list[executor_pb2.ExecuteItem]] = []

    async def execute(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        self.execute_batches.append(list(items))
        return [result(item) for item in items]

    async def prefill(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        self.prefill_batches.append(list(items))
        return [result(item) for item in items]

    async def decode(
        self, items: list[executor_pb2.ExecuteItem]
    ) -> list[executor_pb2.ExecuteResult]:
        self.decode_batches.append(list(items))
        return [result(item) for item in items]

    async def release_blocks(self, block_ids: list[int]) -> None:
        pass

    @property
    def runtime_info(self) -> RuntimeInfo:
        return RuntimeInfo(
            model_type="test",
            dtype="float32",
            block_size=16,
            num_kv_blocks=8,
            num_hidden_layers=1,
            num_kv_heads=1,
            head_dim=1,
            total_memory_bytes=0,
            available_memory_bytes=0,
            kv_cache_bytes=0,
        )


class ExecuteServiceTest(unittest.IsolatedAsyncioTestCase):
    async def test_mixed_batch_is_forwarded_intact(self):
        runner = RecordingRunner()
        cfg = ExecutorConfig(
            runner=RunnerConfig(
                executor_id="executor-test",
                model_id="model_test",
                model_path="unused-in-service-test",
                model_type="test",
            ),
            runtime=RuntimeConfig(
                device="cpu",
                tensor_parallel_size=1,
                gpu_memory_utilization=0.9,
            ),
        )
        service = ExecuteServiceImpl(runner, cfg)
        epoch = service.runtime_epoch

        request = executor_pb2.ExecuteBatchRequest(
            batch_id="batch-id",
            runtime_epoch=epoch,
            items=[
                executor_pb2.ExecuteItem(
                    work_id="d1",
                    request_id="r1",
                    phase=core_pb2.WORK_PHASE_DECODE,
                ),
                executor_pb2.ExecuteItem(
                    work_id="p1",
                    request_id="r2",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                ),
                executor_pb2.ExecuteItem(
                    work_id="p2",
                    request_id="r3",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                ),
            ],
        )
        ctx = cast(
            RequestContext[
                executor_pb2.ExecuteBatchRequest,
                executor_pb2.ExecuteBatchResponse,
            ],
            Mock(spec=RequestContext),
        )
        response = await service.execute_batch(request, ctx)
        self.assertEqual(
            [item.work_id for item in runner.execute_batches[0]],
            ["d1", "p1", "p2"],
        )
        self.assertEqual(
            [item.work_id for item in response.results],
            ["d1", "p1", "p2"],
        )


if __name__ == "__main__":
    unittest.main()
