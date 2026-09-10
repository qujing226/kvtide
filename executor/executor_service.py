import asyncio
import re
import secrets
from urllib.parse import urlparse

import torch

from connectrpc.code import Code
from connectrpc.errors import ConnectError
from connectrpc.request import RequestContext

from kvtide.v1 import executor_connect, executor_pb2
from runner import ModelRunner
from runtime.kv_transfer import KVTransferRuntime
from setting import ExecutorConfig


MAX_KV_TRANSFER_BYTES = 64 * 1024 * 1024


class ExecuteServiceImpl(executor_connect.ExecutorService):
    def __init__(
        self, runner: ModelRunner, cfg: ExecutorConfig,
        *, max_transfer_bytes: int = MAX_KV_TRANSFER_BYTES,
    ):
        if max_transfer_bytes <= 0:
            raise ValueError("max_transfer_bytes must be positive")
        self.runner = runner
        self.cfg = cfg
        # Avoid the proto3 default value so an omitted epoch never looks valid.
        self.runtime_epoch = secrets.randbelow(2**32 - 1) + 1
        self.max_transfer_bytes = max_transfer_bytes
        # v1 serializes cache users with snapshot/import; no concurrent CUDA streams.
        self._cache_lock = asyncio.Lock()
        self._cache_failed = False

    async def execute_batch(
        self, request, ctx: RequestContext
    ) -> executor_pb2.ExecuteBatchResponse:
        self._validate_epoch(request.runtime_epoch)
        response = executor_pb2.ExecuteBatchResponse(
            batch_id=request.batch_id,
            executor_id=self.cfg.runner.executor_id,
        )
        async with self._cache_lock:
            self._ensure_healthy()
            response.results.extend(await self.runner.execute(list(request.items)))
        return response

    async def get_runtime(
        self, request: executor_pb2.GetRuntimeRequest, ctx: RequestContext
    ) -> executor_pb2.GetRuntimeResponse:
        info = self.runner.runtime_info
        self._ensure_healthy()
        transfer = self.runner.kv_transfer
        compatibility = transfer.compatibility if transfer is not None else None
        return executor_pb2.GetRuntimeResponse(
            executor_id=self.cfg.runner.executor_id,
            runtime_epoch=self.runtime_epoch,
            model_id=self.cfg.runner.model_id,
            model_type=info.model_type,
            dtype=info.dtype,
            device_type=self.cfg.runtime.device,
            tensor_parallel_size=self.cfg.runtime.tensor_parallel_size,
            block_size=info.block_size,
            num_kv_blocks=info.num_kv_blocks,
            num_hidden_layers=info.num_hidden_layers,
            num_kv_heads=info.num_kv_heads,
            head_dim=info.head_dim,
            total_memory_bytes=info.total_memory_bytes,
            available_memory_bytes=info.available_memory_bytes,
            kv_cache_bytes=info.kv_cache_bytes,
            model_revision=compatibility.model_revision if compatibility else "",
            kv_layout_version=compatibility.kv_layout_version if compatibility else 0,
            kv_compatibility_id=compatibility.kv_compatibility_id if compatibility else "",
            transfer_endpoint=self.cfg.runtime.transfer_endpoint if compatibility else "",
        )

    async def release_blocks(
        self, request: executor_pb2.ReleaseBlocksRequest, ctx: RequestContext
    ) -> executor_pb2.ReleaseBlocksResponse:
        self._validate_epoch(request.runtime_epoch)
        async with self._cache_lock:
            self._ensure_healthy()
            await self.runner.release_blocks(list(request.block_ids))
        return executor_pb2.ReleaseBlocksResponse()

    async def trigger_kv_push(
        self,
        request: executor_pb2.TriggerKVPushRequest,
        ctx: RequestContext,
    ) -> executor_pb2.TriggerKVPushResponse:
        """Snapshot Engine-selected source blocks, then synchronously push to B."""
        async with self._cache_lock:
            self._ensure_healthy()
            transfer = self._validate_trigger(request)
            try:
                key, value = transfer.export_blocks(list(request.source_block_ids))
            except ValueError as exc:
                raise ConnectError(Code.INVALID_ARGUMENT, str(exc)) from exc

        push = executor_pb2.PushKVRequest(
            transfer_id=request.transfer_id,
            source_executor_id=request.source_executor_id,
            source_runtime_epoch=request.source_runtime_epoch,
            destination_executor_id=request.destination_executor_id,
            destination_runtime_epoch=request.destination_runtime_epoch,
            kv_compatibility_id=request.kv_compatibility_id,
            block_hashes=request.block_hashes,
            destination_block_ids=request.destination_block_ids,
            key_data=key,
            value_data=value,
        )
        timeout_ms = None
        if ctx is not None and ctx.timeout_ms is not None:
            timeout_ms = int(ctx.timeout_ms)
            if timeout_ms <= 0:
                raise ConnectError(Code.DEADLINE_EXCEEDED, "transfer deadline elapsed")
        response = await self._send_push_kv(
            request.destination_transfer_endpoint,
            push,
            timeout_ms,
        )
        if (
            response.transfer_id != request.transfer_id
            or response.destination_executor_id != request.destination_executor_id
            or response.destination_runtime_epoch != request.destination_runtime_epoch
        ):
            raise ConnectError(
                Code.DATA_LOSS,
                "destination completion response mismatch; transfer outcome is unknown",
            )
        return executor_pb2.TriggerKVPushResponse(
            transfer_id=request.transfer_id,
            source_executor_id=self.cfg.runner.executor_id,
            source_runtime_epoch=self.runtime_epoch,
            destination_executor_id=response.destination_executor_id,
            destination_runtime_epoch=response.destination_runtime_epoch,
        )

    async def _send_push_kv(
        self,
        endpoint: str,
        request: executor_pb2.PushKVRequest,
        timeout_ms: int | None,
    ) -> executor_pb2.PushKVResponse:
        async with executor_connect.ExecutorServiceClient(
            endpoint,
            send_compression=None,
            read_max_bytes=64 * 1024,
        ) as client:
            return await client.push_kv(request, timeout_ms=timeout_ms)

    async def push_kv(
        self, request: executor_pb2.PushKVRequest, ctx: RequestContext,
    ) -> executor_pb2.PushKVResponse:
        async with self._cache_lock:
            transfer = self._validate_push(request)
            blocks = list(request.destination_block_ids)
            try:
                transfer.validate_destination(blocks)
            except ValueError as exc:
                raise ConnectError(Code.FAILED_PRECONDITION, str(exc)) from exc
            try:
                if len(request.key_data) + len(request.value_data) > self.max_transfer_bytes:
                    raise ConnectError(Code.RESOURCE_EXHAUSTED, "KV transfer exceeds the configured byte limit")
                expected = transfer.payload_bytes(len(blocks))
                if len(request.key_data) != expected or len(request.value_data) != expected:
                    raise ConnectError(
                        Code.INVALID_ARGUMENT, f"each KV payload must have exactly {expected} bytes"
                    )
                # Blocking import includes device completion, before returning READY.
                transfer.import_blocks(blocks, request.key_data, request.value_data)
            except Exception as exc:
                try:
                    transfer.invalidate_blocks(blocks)
                except Exception as rollback_error:
                    # A failed device must not continue serving partially valid KV.
                    self._cache_failed = True
                    raise ConnectError(
                        Code.INTERNAL, "KV rollback failed; restart this executor"
                    ) from rollback_error
                if isinstance(exc, ConnectError):
                    raise
                if isinstance(exc, torch.OutOfMemoryError):
                    raise ConnectError(Code.RESOURCE_EXHAUSTED, "insufficient memory for KV import") from exc
                if isinstance(exc, ValueError):
                    raise ConnectError(Code.INVALID_ARGUMENT, str(exc)) from exc
                raise ConnectError(Code.INTERNAL, "KV import failed") from exc
            return executor_pb2.PushKVResponse(
                transfer_id=request.transfer_id,
                destination_executor_id=self.cfg.runner.executor_id,
                destination_runtime_epoch=self.runtime_epoch,
            )

    def _validate_trigger(
        self, request: executor_pb2.TriggerKVPushRequest
    ) -> KVTransferRuntime:
        self._validate_epoch(request.source_runtime_epoch)
        if request.source_executor_id != self.cfg.runner.executor_id:
            raise ConnectError(Code.FAILED_PRECONDITION, "source executor ID mismatch")
        transfer = self._validate_common_transfer(
            transfer_id=request.transfer_id,
            source_executor_id=request.source_executor_id,
            source_runtime_epoch=request.source_runtime_epoch,
            destination_executor_id=request.destination_executor_id,
            destination_runtime_epoch=request.destination_runtime_epoch,
            compatibility_id=request.kv_compatibility_id,
            block_hashes=request.block_hashes,
            destination_block_ids=request.destination_block_ids,
        )
        source_blocks = request.source_block_ids
        if len(source_blocks) != len(request.destination_block_ids):
            raise ConnectError(
                Code.INVALID_ARGUMENT,
                "source, hash, and destination block counts must match",
            )
        if len(source_blocks) != len(set(source_blocks)):
            raise ConnectError(Code.INVALID_ARGUMENT, "source block IDs must be unique")
        endpoint = urlparse(request.destination_transfer_endpoint)
        if (
            endpoint.scheme not in {"http", "https"}
            or not endpoint.hostname
            or endpoint.username is not None
            or endpoint.password is not None
            or endpoint.query
            or endpoint.fragment
        ):
            raise ConnectError(Code.INVALID_ARGUMENT, "invalid destination transfer endpoint")
        return transfer

    def _validate_push(self, request: executor_pb2.PushKVRequest) -> KVTransferRuntime:
        self._ensure_healthy()
        self._validate_epoch(request.destination_runtime_epoch)
        if request.destination_executor_id != self.cfg.runner.executor_id:
            raise ConnectError(Code.FAILED_PRECONDITION, "destination executor ID mismatch")
        transfer = self._validate_common_transfer(
            transfer_id=request.transfer_id,
            source_executor_id=request.source_executor_id,
            source_runtime_epoch=request.source_runtime_epoch,
            destination_executor_id=request.destination_executor_id,
            destination_runtime_epoch=request.destination_runtime_epoch,
            compatibility_id=request.kv_compatibility_id,
            block_hashes=request.block_hashes,
            destination_block_ids=request.destination_block_ids,
        )
        if any(block >= transfer.cache.num_blocks for block in request.destination_block_ids):
            raise ConnectError(Code.INVALID_ARGUMENT, "invalid destination block ID")
        return transfer

    def _validate_common_transfer(
        self,
        *,
        transfer_id: str,
        source_executor_id: str,
        source_runtime_epoch: int,
        destination_executor_id: str,
        destination_runtime_epoch: int,
        compatibility_id: str,
        block_hashes,
        destination_block_ids,
    ) -> KVTransferRuntime:
        self._ensure_healthy()
        transfer = self.runner.kv_transfer
        if transfer is None:
            raise ConnectError(Code.UNIMPLEMENTED, "this runner does not support KV transfer")
        if (
            not transfer_id or len(transfer_id) > 128
            or not source_executor_id or len(source_executor_id) > 128
            or source_runtime_epoch == 0
            or not destination_executor_id or len(destination_executor_id) > 128
            or destination_runtime_epoch == 0
        ):
            raise ConnectError(Code.INVALID_ARGUMENT, "transfer identity is missing or invalid")
        if (
            not compatibility_id
            or compatibility_id != transfer.compatibility.kv_compatibility_id
        ):
            raise ConnectError(Code.FAILED_PRECONDITION, "KV compatibility mismatch")
        blocks = destination_block_ids
        if not blocks or len(blocks) != len(set(blocks)):
            raise ConnectError(Code.INVALID_ARGUMENT, "destination block IDs must be nonempty and unique")
        if len(block_hashes) != len(blocks) or any(
            not re.fullmatch(r"[0-9a-f]{64}", block_hash) for block_hash in block_hashes
        ):
            raise ConnectError(
                Code.INVALID_ARGUMENT,
                "block_hashes must contain one SHA-256 hash per destination block",
            )
        if 2 * transfer.payload_bytes(len(blocks)) > self.max_transfer_bytes:
            raise ConnectError(Code.RESOURCE_EXHAUSTED, "KV transfer exceeds the configured byte limit")
        return transfer

    def _ensure_healthy(self) -> None:
        if self._cache_failed:
            raise ConnectError(Code.FAILED_PRECONDITION, "KV cache failed; restart this executor")

    def _validate_epoch(self, epoch: int) -> None:
        if epoch != self.runtime_epoch:
            raise ConnectError(
                Code.FAILED_PRECONDITION,
                f"runtime epoch mismatch: got {epoch}, want {self.runtime_epoch}",
            )
