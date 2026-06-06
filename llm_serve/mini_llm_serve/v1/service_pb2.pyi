from mini_llm_serve.v1 import core_pb2 as _core_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GenerateRequest(_message.Message):
    __slots__ = ("request_id", "user_id", "model", "prompt", "max_tokens", "timeout_ms", "labels")
    class LabelsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    MAX_TOKENS_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MS_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    user_id: str
    model: str
    prompt: str
    max_tokens: int
    timeout_ms: int
    labels: _containers.ScalarMap[str, str]
    def __init__(self, request_id: _Optional[str] = ..., user_id: _Optional[str] = ..., model: _Optional[str] = ..., prompt: _Optional[str] = ..., max_tokens: _Optional[int] = ..., timeout_ms: _Optional[int] = ..., labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class GenerateResponse(_message.Message):
    __slots__ = ("request_id", "output_text", "finish_reason", "usage", "timing", "batch", "executor_id", "error_message")
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TEXT_FIELD_NUMBER: _ClassVar[int]
    FINISH_REASON_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    TIMING_FIELD_NUMBER: _ClassVar[int]
    BATCH_FIELD_NUMBER: _ClassVar[int]
    EXECUTOR_ID_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    output_text: str
    finish_reason: _core_pb2.FinishReason
    usage: _core_pb2.Usage
    timing: _core_pb2.Timing
    batch: _core_pb2.BatchInfo
    executor_id: str
    error_message: str
    def __init__(self, request_id: _Optional[str] = ..., output_text: _Optional[str] = ..., finish_reason: _Optional[_Union[_core_pb2.FinishReason, str]] = ..., usage: _Optional[_Union[_core_pb2.Usage, _Mapping]] = ..., timing: _Optional[_Union[_core_pb2.Timing, _Mapping]] = ..., batch: _Optional[_Union[_core_pb2.BatchInfo, _Mapping]] = ..., executor_id: _Optional[str] = ..., error_message: _Optional[str] = ...) -> None: ...

class GenerateResponseChunk(_message.Message):
    __slots__ = ("request_id", "index", "delta_text", "done", "finish_reason", "usage", "error_message")
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    DELTA_TEXT_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    FINISH_REASON_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    index: int
    delta_text: str
    done: bool
    finish_reason: _core_pb2.FinishReason
    usage: _core_pb2.Usage
    error_message: str
    def __init__(self, request_id: _Optional[str] = ..., index: _Optional[int] = ..., delta_text: _Optional[str] = ..., done: _Optional[bool] = ..., finish_reason: _Optional[_Union[_core_pb2.FinishReason, str]] = ..., usage: _Optional[_Union[_core_pb2.Usage, _Mapping]] = ..., error_message: _Optional[str] = ...) -> None: ...

class HealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthResponse(_message.Message):
    __slots__ = ("status",)
    STATUS_FIELD_NUMBER: _ClassVar[int]
    status: str
    def __init__(self, status: _Optional[str] = ...) -> None: ...

class GetRuntimeStatsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetRuntimeStatsResponse(_message.Message):
    __slots__ = ("prefill_queue_len", "decode_queue_len", "inflight_requests", "inflight_batches", "busy_executors", "idle_executors")
    PREFILL_QUEUE_LEN_FIELD_NUMBER: _ClassVar[int]
    DECODE_QUEUE_LEN_FIELD_NUMBER: _ClassVar[int]
    INFLIGHT_REQUESTS_FIELD_NUMBER: _ClassVar[int]
    INFLIGHT_BATCHES_FIELD_NUMBER: _ClassVar[int]
    BUSY_EXECUTORS_FIELD_NUMBER: _ClassVar[int]
    IDLE_EXECUTORS_FIELD_NUMBER: _ClassVar[int]
    prefill_queue_len: int
    decode_queue_len: int
    inflight_requests: int
    inflight_batches: int
    busy_executors: int
    idle_executors: int
    def __init__(self, prefill_queue_len: _Optional[int] = ..., decode_queue_len: _Optional[int] = ..., inflight_requests: _Optional[int] = ..., inflight_batches: _Optional[int] = ..., busy_executors: _Optional[int] = ..., idle_executors: _Optional[int] = ...) -> None: ...
