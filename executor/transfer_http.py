"""Receive-side limits for the experimental, uncompressed Connect PushKV RPC."""

import json

from connectrpc.code import Code
from connectrpc.errors import ConnectError

from executor_service import ExecuteServiceImpl
from kvtide.v1.executor_connect import ExecutorServiceASGIApplication


def create_executor_app(service: ExecuteServiceImpl, *, max_wire_bytes: int | None = None):
    limit = max_wire_bytes if max_wire_bytes is not None else service.max_transfer_bytes + 64 * 1024
    if limit <= 0:
        raise ValueError("max_wire_bytes must be positive")
    application = ExecutorServiceASGIApplication(service, read_max_bytes=limit)

    async def app(scope, receive, send):
        if scope["type"] != "http" or scope.get("path") != "/kvtide.v1.ExecutorService/PushKV":
            await application(scope, receive, send)
            return

        headers = scope.get("headers", [])
        compressed = any(
            key.lower() in {b"content-encoding", b"grpc-encoding", b"connect-content-encoding"}
            and value.strip().lower() not in {b"", b"identity"}
            for key, value in headers
        )
        content_types = [v.split(b";", 1)[0].strip().lower() for k, v in headers if k.lower() == b"content-type"]
        # Framed gRPC compression is intentionally not part of this v1 receiver.
        if compressed or content_types not in ([b"application/proto"], [b"application/json"]):
            body = json.dumps({
                "code": "unimplemented",
                "message": "PushKV v1 requires uncompressed Connect protobuf or JSON",
            }).encode()
            await send({"type": "http.response.start", "status": 415,
                        "headers": [(b"content-type", b"application/json")]})
            await send({"type": "http.response.body", "body": body})
            return

        total = 0

        async def limited_receive():
            nonlocal total
            message = await receive()
            if message["type"] == "http.request":
                total += len(message.get("body", b""))
                if total > limit:
                    # Raise before forwarding the overflowing chunk to Connect's
                    # buffering/decoding layer. The framework formats the RPC error.
                    raise ConnectError(Code.RESOURCE_EXHAUSTED, "PushKV request body exceeds byte limit")
            return message

        await application(scope, limited_receive, send)

    return app
