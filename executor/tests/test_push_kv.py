import asyncio
import copy
import unittest
from dataclasses import replace
from unittest.mock import patch

import torch
from connectrpc.code import Code
from connectrpc.errors import ConnectError
from executor_service import ExecuteServiceImpl
from kvtide.v1 import core_pb2, executor_pb2
from runner.transformers import Runner
from setting import ExecutorConfig, RunnerConfig, RuntimeConfig
from test_executor_service import RecordingRunner
from test_kv_transfer import build_transfer
from transfer_http import create_executor_app
from transformers import Qwen3Config, Qwen3ForCausalLM


async def asgi_push(service, request, *, encoding=None, max_wire_bytes=None):
    app = create_executor_app(service, max_wire_bytes=max_wire_bytes)
    messages = []
    received = False
    finished = asyncio.Event()

    async def receive():
        nonlocal received
        if not received:
            received = True
            return {
                "type": "http.request",
                "body": request.SerializeToString(),
                "more_body": False,
            }
        await finished.wait()
        return {"type": "http.disconnect"}

    async def send(message):
        messages.append(message)
        if message["type"] == "http.response.body" and not message.get(
            "more_body", False
        ):
            finished.set()

    path = "/kvtide.v1.ExecutorService/PushKV"
    scope = {
        "type": "http",
        "asgi": {"version": "3.0"},
        "http_version": "1.1",
        "method": "POST",
        "scheme": "http",
        "path": path,
        "raw_path": path.encode(),
        "root_path": "",
        "query_string": b"",
        "headers": [
            (b"content-type", b"application/proto"),
            (b"connect-protocol-version", b"1"),
        ],
        "server": ("test", 80),
        "client": ("test", 1),
    }
    if encoding:
        scope["headers"].append((b"content-encoding", encoding))
    await asyncio.wait_for(app(scope, receive, send), timeout=5)
    status = next(m["status"] for m in messages if m["type"] == "http.response.start")
    body = b"".join(m.get("body", b"") for m in messages)
    return status, body


class TransferRunner(RecordingRunner):
    def __init__(self):
        super().__init__()
        self._kv_transfer = build_transfer()

    async def release_blocks(self, block_ids):
        self.kv_transfer.cache.release(block_ids)

    @property
    def runtime_info(self):
        c = self.kv_transfer.cache
        return replace(
            super().runtime_info,
            block_size=c.block_size,
            num_kv_blocks=c.num_blocks,
            num_hidden_layers=c.num_layers,
            num_kv_heads=c.num_kv_heads,
            head_dim=c.head_dim,
        )


class PushKVTest(unittest.IsolatedAsyncioTestCase):
    def setUp(self):
        self.runner = TransferRunner()
        cfg = ExecutorConfig(
            RunnerConfig("destination", "tiny", "unused", "test"),
            RuntimeConfig(
                "cpu",
                1,
                0.9,
                536870912,
                transfer_endpoint="http://destination:19991",
            ),
        )
        self.service = ExecuteServiceImpl(self.runner, cfg)
        self.source = build_transfer()
        self.source.cache.key_cache.fill_(1)
        self.source.cache.value_cache.fill_(2)
        self.source.cache.valid_slots.fill_(True)
        key, value = self.source.export_blocks([2, 0])
        self.hashes = ["a" * 64, "b" * 64]
        self.request = executor_pb2.PushKVRequest(
            transfer_id="copy-1",
            source_executor_id="source",
            source_runtime_epoch=123,
            destination_executor_id="destination",
            destination_runtime_epoch=self.service.runtime_epoch,
            kv_compatibility_id=self.source.compatibility.kv_compatibility_id,
            block_hashes=self.hashes,
            destination_block_ids=[1, 3],
            key_data=key,
            value_data=value,
        )

    async def test_runtime_advertises_actual_compatibility(self):
        response = await self.service.get_runtime(
            executor_pb2.GetRuntimeRequest(), None
        )
        self.assertEqual(
            response.model_revision, self.source.compatibility.model_revision
        )
        self.assertEqual(response.kv_compatibility_id, self.request.kv_compatibility_id)
        self.assertEqual(response.kv_layout_version, 1)
        self.assertEqual(response.transfer_endpoint, "http://destination:19991")

    async def test_trigger_forwards_engine_hashes_and_waits_for_destination(self):
        self.runner.kv_transfer.cache.key_cache.fill_(3)
        self.runner.kv_transfer.cache.value_cache.fill_(4)
        self.runner.kv_transfer.cache.valid_slots.fill_(True)
        request = executor_pb2.TriggerKVPushRequest(
            transfer_id="copy-trigger",
            source_executor_id="destination",
            source_runtime_epoch=self.service.runtime_epoch,
            source_block_ids=[2, 0],
            destination_executor_id="remote",
            destination_runtime_epoch=456,
            destination_transfer_endpoint="http://remote:19991",
            kv_compatibility_id=self.source.compatibility.kv_compatibility_id,
            block_hashes=self.hashes,
            destination_block_ids=[5, 7],
        )
        sent = []

        async def send(endpoint, push, timeout_ms):
            sent.append((endpoint, push, timeout_ms))
            return executor_pb2.PushKVResponse(
                transfer_id=push.transfer_id,
                destination_executor_id=push.destination_executor_id,
                destination_runtime_epoch=push.destination_runtime_epoch,
            )

        with patch.object(self.service, "_send_push_kv", side_effect=send):
            response = await self.service.trigger_kv_push(request, None)

        self.assertEqual(response.transfer_id, "copy-trigger")
        self.assertEqual(response.source_runtime_epoch, self.service.runtime_epoch)
        self.assertEqual(response.destination_executor_id, "remote")
        self.assertEqual(len(sent), 1)
        endpoint, push, timeout_ms = sent[0]
        self.assertEqual(endpoint, "http://remote:19991")
        self.assertEqual(list(push.block_hashes), self.hashes)
        self.assertEqual(list(push.destination_block_ids), [5, 7])
        self.assertTrue(
            (torch.frombuffer(bytearray(push.key_data), dtype=torch.float32) == 3).all()
        )
        self.assertTrue(
            (
                torch.frombuffer(bytearray(push.value_data), dtype=torch.float32) == 4
            ).all()
        )
        self.assertIsNone(timeout_ms)

    async def test_trigger_rejects_invalid_engine_metadata_before_export(self):
        request = executor_pb2.TriggerKVPushRequest(
            transfer_id="bad-trigger",
            source_executor_id="destination",
            source_runtime_epoch=self.service.runtime_epoch,
            source_block_ids=[2, 0],
            destination_executor_id="remote",
            destination_runtime_epoch=456,
            destination_transfer_endpoint="http://remote:19991",
            kv_compatibility_id=self.source.compatibility.kv_compatibility_id,
            block_hashes=["bad"],
            destination_block_ids=[5, 7],
        )
        with patch.object(self.runner.kv_transfer, "export_blocks") as export:
            with self.assertRaises(ConnectError) as error:
                await self.service.trigger_kv_push(request, None)
        self.assertEqual(error.exception.code, Code.INVALID_ARGUMENT)
        export.assert_not_called()

    async def test_push_imports_without_executor_side_reservation(self):
        response = await self.service.push_kv(self.request, None)
        self.assertEqual(response.transfer_id, "copy-1")
        self.assertEqual(response.destination_runtime_epoch, self.service.runtime_epoch)
        self.source.cache.release([2, 0])
        self.source.cache.key_cache.zero_()
        k, v = self.runner.kv_transfer.cache.export_blocks([1, 3])
        self.assertTrue((k == 1).all().item())
        self.assertTrue((v == 2).all().item())
        with self.assertRaises(ConnectError):
            await self.service.push_kv(self.request, None)
        k, v = self.runner.kv_transfer.cache.export_blocks([1, 3])
        self.assertTrue((k == 1).all().item())
        self.assertTrue((v == 2).all().item())

    async def test_push_rejects_wrong_identity_epoch_compatibility_and_hashes(self):
        for field, wrong in (
            ("destination_executor_id", "elsewhere"),
            ("destination_runtime_epoch", 0),
            ("source_runtime_epoch", 0),
            ("kv_compatibility_id", ""),
        ):
            with self.subTest(field=field):
                request = executor_pb2.PushKVRequest()
                request.CopyFrom(self.request)
                setattr(request, field, wrong)
                with self.assertRaises(ConnectError):
                    await self.service.push_kv(request, None)
                self.assertFalse(self.runner.kv_transfer.cache.valid_slots.any().item())

        for hashes in (["bad-hash", "b" * 64], ["a" * 64]):
            with self.subTest(hashes=hashes):
                request = executor_pb2.PushKVRequest()
                request.CopyFrom(self.request)
                del request.block_hashes[:]
                request.block_hashes.extend(hashes)
                with self.assertRaises(ConnectError):
                    await self.service.push_kv(request, None)
                self.assertFalse(self.runner.kv_transfer.cache.valid_slots.any().item())

    async def test_truncated_value_is_rejected_before_write(self):
        self.request.value_data = self.request.value_data[:-1]
        with self.assertRaises(ConnectError) as error:
            await self.service.push_kv(self.request, None)
        self.assertEqual(error.exception.code, Code.INVALID_ARGUMENT)
        self.assertFalse(self.runner.kv_transfer.cache.key_cache.any().item())
        self.assertFalse(self.runner.kv_transfer.cache.valid_slots.any().item())

    async def test_import_failure_rolls_back_validity(self):
        def fail(*args):
            self.runner.kv_transfer.cache.valid_slots[:, [1, 3], :] = True
            raise RuntimeError("device copy failed")

        with patch.object(self.runner.kv_transfer, "import_blocks", side_effect=fail):
            with self.assertRaises(ConnectError):
                await self.service.push_kv(self.request, None)
        self.assertFalse(self.runner.kv_transfer.cache.valid_slots.any().item())

    async def test_oversized_transfer_is_rejected(self):
        self.service.max_transfer_bytes = 1
        with self.assertRaises(ConnectError) as error:
            await self.service.push_kv(self.request, None)
        self.assertEqual(error.exception.code, Code.RESOURCE_EXHAUSTED)

    async def test_wire_limit_and_compression_are_rejected_before_import(self):
        status, _ = await asgi_push(self.service, self.request, max_wire_bytes=16)
        self.assertEqual(status, 429)
        status, _ = await asgi_push(self.service, self.request, encoding=b"gzip")
        self.assertEqual(status, 415)
        self.assertFalse(self.runner.kv_transfer.cache.valid_slots.any().item())

    async def test_partially_valid_destination_is_rejected_without_erasing_it(self):
        self.runner.kv_transfer.cache.valid_slots[1, 3, 0] = True
        with self.assertRaises(ConnectError) as error:
            await self.service.push_kv(self.request, None)
        self.assertEqual(error.exception.code, Code.FAILED_PRECONDITION)
        self.assertTrue(self.runner.kv_transfer.cache.valid_slots[1, 3, 0].item())

    async def test_push_waits_for_inflight_execution(self):
        entered = asyncio.Event()
        proceed = asyncio.Event()

        async def execute(items):
            entered.set()
            await proceed.wait()
            return []

        with patch.object(self.runner, "execute", side_effect=execute):
            task = asyncio.create_task(
                self.service.execute_batch(
                    executor_pb2.ExecuteBatchRequest(
                        runtime_epoch=self.service.runtime_epoch
                    ),
                    None,
                )
            )
            await asyncio.wait_for(entered.wait(), 2)
            push_task = asyncio.create_task(self.service.push_kv(self.request, None))
            try:
                await asyncio.sleep(0)
                self.assertFalse(push_task.done())
            finally:
                proceed.set()
                await asyncio.wait_for(asyncio.gather(task, push_task), 2)

    async def test_mock_runner_does_not_support_transfer(self):
        service = ExecuteServiceImpl(RecordingRunner(), self.service.cfg)
        response = await service.get_runtime(executor_pb2.GetRuntimeRequest(), None)
        self.assertEqual(response.kv_compatibility_id, "")
        self.request.destination_runtime_epoch = service.runtime_epoch
        with self.assertRaises(ConnectError) as error:
            await service.push_kv(self.request, None)
        self.assertEqual(error.exception.code, Code.UNIMPLEMENTED)

    async def test_asgi_accepts_engine_allocated_destination(self):
        status, body = await asgi_push(self.service, self.request)
        self.assertEqual(status, 200, body)
        response = executor_pb2.PushKVResponse.FromString(body)
        self.assertEqual(response.transfer_id, "copy-1")

    async def test_rollback_failure_stops_serving(self):
        with (
            patch.object(
                self.runner.kv_transfer,
                "import_blocks",
                side_effect=RuntimeError("failed"),
            ),
            patch.object(
                self.runner.kv_transfer.cache,
                "release",
                side_effect=RuntimeError("failed"),
            ),
        ):
            with self.assertRaises(ConnectError):
                await self.service.push_kv(self.request, None)
        with self.assertRaises(ConnectError):
            await self.service.get_runtime(executor_pb2.GetRuntimeRequest(), None)


class PushKVInferenceTest(unittest.IsolatedAsyncioTestCase):
    async def test_qwen_trigger_push_then_suffix_matches_full_recompute(self):
        model_config = Qwen3Config(
            vocab_size=32,
            hidden_size=16,
            intermediate_size=32,
            num_hidden_layers=2,
            num_attention_heads=2,
            num_key_value_heads=1,
            head_dim=8,
            max_position_embeddings=64,
            eos_token_id=31,
        )
        with torch.random.fork_rng(devices=[]):
            torch.manual_seed(0)
            model = Qwen3ForCausalLM(model_config).eval()

        services = []
        for name in ("source", "destination"):
            cfg = ExecutorConfig(
                RunnerConfig(name, "tiny", "unused", "qwen3"),
                RuntimeConfig(
                    "cpu",
                    1,
                    0.9,
                    32768,
                    transfer_endpoint=f"http://{name}:19991",
                ),
            )
            with (
                patch(
                    "runner.transformers.AutoConfig.from_pretrained",
                    return_value=model_config,
                ),
                patch(
                    "runner.transformers.AutoModelForCausalLM.from_pretrained",
                    return_value=copy.deepcopy(model),
                ),
            ):
                runner = Runner(cfg.runner, cfg.runtime)
            services.append(ExecuteServiceImpl(runner, cfg))
        source, destination = services
        source_info = await source.get_runtime(executor_pb2.GetRuntimeRequest(), None)
        destination_info = await destination.get_runtime(
            executor_pb2.GetRuntimeRequest(), None
        )
        self.assertEqual(
            source_info.kv_compatibility_id, destination_info.kv_compatibility_id
        )
        prefix = [1, 5, 2, 9, 3, 7, 4, 11] * 4
        suffix = [6, 10]

        def batch(service, tokens, past, table, allocated):
            request = executor_pb2.ExecuteBatchRequest(
                runtime_epoch=service.runtime_epoch
            )
            item = request.items.add(
                work_id="work",
                request_id="request",
                phase=core_pb2.WORK_PHASE_PREFILL,
                token_ids=tokens,
                computed_tokens=past,
                num_new_tokens=len(tokens),
                sample=True,
            )
            item.kv_blocks.block_size = 16
            item.kv_blocks.block_table.extend(table)
            item.kv_blocks.allocated_blocks.extend(allocated)
            return request

        await source.execute_batch(batch(source, prefix, 0, [2, 0], [2, 0]), None)
        trigger = executor_pb2.TriggerKVPushRequest(
            transfer_id="qwen-copy",
            source_executor_id="source",
            source_runtime_epoch=source.runtime_epoch,
            source_block_ids=[2, 0],
            destination_executor_id="destination",
            destination_runtime_epoch=destination.runtime_epoch,
            destination_transfer_endpoint="http://destination:19991",
            kv_compatibility_id=source_info.kv_compatibility_id,
            block_hashes=["a" * 64, "b" * 64],
            destination_block_ids=[1, 3],
        )

        async def send(_endpoint, push, _timeout_ms):
            status, body = await asgi_push(destination, push)
            self.assertEqual(status, 200, body)
            return executor_pb2.PushKVResponse.FromString(body)

        with patch.object(source, "_send_push_kv", side_effect=send):
            response = await source.trigger_kv_push(trigger, None)
        self.assertEqual(response.destination_executor_id, "destination")

        await source.release_blocks(
            executor_pb2.ReleaseBlocksRequest(
                runtime_epoch=source.runtime_epoch,
                block_ids=[2, 0],
            ),
            None,
        )
        source.runner.kv_cache.key_cache.zero_()
        source.runner.kv_cache.value_cache.zero_()

        logits = []

        def observe(_model, _inputs, output):
            logits.append(output.logits[:, -1, :].clone())

        hook = destination.runner.model.register_forward_hook(observe)
        try:
            result = await destination.execute_batch(
                batch(destination, suffix, len(prefix), [1, 3, 4], [4]),
                None,
            )
        finally:
            hook.remove()
        with torch.inference_mode():
            reference = model(
                input_ids=torch.tensor([prefix + suffix]),
                use_cache=False,
                logits_to_keep=1,
            ).logits[:, -1, :]
        torch.testing.assert_close(logits[0], reference, rtol=1e-4, atol=1e-5)
        self.assertEqual(result.results[0].token_id, int(reference.argmax(-1).item()))
