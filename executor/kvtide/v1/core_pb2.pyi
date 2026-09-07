from vendor.daotl.protoc_gen_go_enums import ext_pb2 as _ext_pb2
from vendor.daotl.protoc_gen_go_string_consts import ext_pb2 as _ext_pb2_1
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class FinishReason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    FINISH_REASON_UNSPECIFIED: _ClassVar[FinishReason]
    FINISH_REASON_STOP: _ClassVar[FinishReason]
    FINISH_REASON_LENGTH: _ClassVar[FinishReason]
    FINISH_REASON_TIMEOUT: _ClassVar[FinishReason]
    FINISH_REASON_ERROR: _ClassVar[FinishReason]

class WorkPhase(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    WORK_PHASE_PREFILL: _ClassVar[WorkPhase]
    WORK_PHASE_DECODE: _ClassVar[WorkPhase]

class EventType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EVENT_TYPE_PREFILL_CHUNK: _ClassVar[EventType]
    EVENT_TYPE_PREFILL_FINISHED: _ClassVar[EventType]
    EVENT_TYPE_DECODE_CHUNK: _ClassVar[EventType]
    EVENT_TYPE_REQUEST_FINISHED: _ClassVar[EventType]
    EVENT_TYPE_REQUEST_FAILED: _ClassVar[EventType]
    EVENT_TYPE_REQUEST_CANCELED: _ClassVar[EventType]
FINISH_REASON_UNSPECIFIED: FinishReason
FINISH_REASON_STOP: FinishReason
FINISH_REASON_LENGTH: FinishReason
FINISH_REASON_TIMEOUT: FinishReason
FINISH_REASON_ERROR: FinishReason
WORK_PHASE_PREFILL: WorkPhase
WORK_PHASE_DECODE: WorkPhase
EVENT_TYPE_PREFILL_CHUNK: EventType
EVENT_TYPE_PREFILL_FINISHED: EventType
EVENT_TYPE_DECODE_CHUNK: EventType
EVENT_TYPE_REQUEST_FINISHED: EventType
EVENT_TYPE_REQUEST_FAILED: EventType
EVENT_TYPE_REQUEST_CANCELED: EventType

class Usage(_message.Message):
    __slots__ = ("input_tokens", "output_tokens", "total_tokens")
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    input_tokens: int
    output_tokens: int
    total_tokens: int
    def __init__(self, input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., total_tokens: _Optional[int] = ...) -> None: ...

class Timing(_message.Message):
    __slots__ = ("queue_ms", "batch_wait_ms", "execution_ms", "total_ms")
    QUEUE_MS_FIELD_NUMBER: _ClassVar[int]
    BATCH_WAIT_MS_FIELD_NUMBER: _ClassVar[int]
    EXECUTION_MS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_MS_FIELD_NUMBER: _ClassVar[int]
    queue_ms: int
    batch_wait_ms: int
    execution_ms: int
    total_ms: int
    def __init__(self, queue_ms: _Optional[int] = ..., batch_wait_ms: _Optional[int] = ..., execution_ms: _Optional[int] = ..., total_ms: _Optional[int] = ...) -> None: ...

class BatchInfo(_message.Message):
    __slots__ = ("batch_id", "batch_size")
    BATCH_ID_FIELD_NUMBER: _ClassVar[int]
    BATCH_SIZE_FIELD_NUMBER: _ClassVar[int]
    batch_id: str
    batch_size: int
    def __init__(self, batch_id: _Optional[str] = ..., batch_size: _Optional[int] = ...) -> None: ...
