import asyncio
import socket
import unittest
from contextlib import asynccontextmanager

import uvicorn
from executor_service import ExecuteServiceImpl
from kvtide.v1 import executor_connect, executor_pb2
from setting import ExecutorConfig, RunnerConfig, RuntimeConfig
from test_push_kv import TransferRunner
from transfer_http import create_executor_app


@asynccontextmanager
async def serve(app):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("127.0.0.1", 0))
    host, port = sock.getsockname()
    server = uvicorn.Server(
        uvicorn.Config(
            app,
            log_level="error",
            access_log=False,
            lifespan="off",
        )
    )
    task = asyncio.create_task(server.serve(sockets=[sock]))
    try:
        while not server.started:
            if task.done():
                await task
            await asyncio.sleep(0.01)
        yield f"http://{host}:{port}"
    finally:
        server.should_exit = True
        await asyncio.wait_for(task, timeout=5)


def build_service(executor_id):
    runner = TransferRunner()
    cfg = ExecutorConfig(
        RunnerConfig(executor_id, "tiny", "unused", "test"),
        RuntimeConfig(
            "cpu",
            1,
            0.9,
            536870912,
            transfer_endpoint=f"http://{executor_id}.invalid",
        ),
    )
    return ExecuteServiceImpl(runner, cfg)


class KVTransferNetworkTest(unittest.IsolatedAsyncioTestCase):
    async def test_engine_to_source_to_destination_over_real_http(self):
        source = build_service("source")
        destination = build_service("destination")
        source.runner.kv_transfer.cache.key_cache.fill_(1)
        source.runner.kv_transfer.cache.value_cache.fill_(2)
        source.runner.kv_transfer.cache.valid_slots.fill_(True)

        source_app = create_executor_app(source)
        destination_app = create_executor_app(destination)
        async with (
            serve(source_app) as source_endpoint,
            serve(destination_app) as destination_endpoint,
        ):
            request = executor_pb2.TriggerKVPushRequest(
                transfer_id="network-copy",
                source_executor_id="source",
                source_runtime_epoch=source.runtime_epoch,
                source_block_ids=[2, 0],
                destination_executor_id="destination",
                destination_runtime_epoch=destination.runtime_epoch,
                destination_transfer_endpoint=destination_endpoint,
                kv_compatibility_id=(
                    source.runner.kv_transfer.compatibility.kv_compatibility_id
                ),
                block_hashes=["a" * 64, "b" * 64],
                destination_block_ids=[1, 3],
            )
            async with executor_connect.ExecutorServiceClient(
                source_endpoint
            ) as client:
                response = await client.trigger_kv_push(request, timeout_ms=5000)

        self.assertEqual(response.transfer_id, "network-copy")
        self.assertEqual(response.source_executor_id, "source")
        self.assertEqual(response.destination_executor_id, "destination")
        key, value = destination.runner.kv_transfer.cache.export_blocks([1, 3])
        self.assertTrue((key == 1).all().item())
        self.assertTrue((value == 2).all().item())
