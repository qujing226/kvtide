from mini_llm_serve.v1 import core_pb2 as _core_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ExecuteBatchRequest(_message.Message):
    __slots__ = ("batch_id", "items")
    BATCH_ID_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    batch_id: str
    items: _containers.RepeatedCompositeFieldContainer[ExecuteItem]
    def __init__(self, batch_id: _Optional[str] = ..., items: _Optional[_Iterable[_Union[ExecuteItem, _Mapping]]] = ...) -> None: ...

class ExecuteBatchResponse(_message.Message):
    __slots__ = ("batch_id", "executor_id", "results")
    BATCH_ID_FIELD_NUMBER: _ClassVar[int]
    EXECUTOR_ID_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    batch_id: str
    executor_id: str
    results: _containers.RepeatedCompositeFieldContainer[ExecuteResult]
    def __init__(self, batch_id: _Optional[str] = ..., executor_id: _Optional[str] = ..., results: _Optional[_Iterable[_Union[ExecuteResult, _Mapping]]] = ...) -> None: ...

class ExecuteItem(_message.Message):
    __slots__ = ("work_id", "request_id", "phase", "prompt", "has_prompt", "prompt_tokens", "computed_tokens", "generated_tokens", "num_new_tokens")
    WORK_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    PHASE_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    HAS_PROMPT_FIELD_NUMBER: _ClassVar[int]
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPUTED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    GENERATED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    NUM_NEW_TOKENS_FIELD_NUMBER: _ClassVar[int]
    work_id: str
    request_id: str
    phase: _core_pb2.WorkPhase
    prompt: str
    has_prompt: bool
    prompt_tokens: int
    computed_tokens: int
    generated_tokens: int
    num_new_tokens: int
    def __init__(self, work_id: _Optional[str] = ..., request_id: _Optional[str] = ..., phase: _Optional[_Union[_core_pb2.WorkPhase, str]] = ..., prompt: _Optional[str] = ..., has_prompt: _Optional[bool] = ..., prompt_tokens: _Optional[int] = ..., computed_tokens: _Optional[int] = ..., generated_tokens: _Optional[int] = ..., num_new_tokens: _Optional[int] = ...) -> None: ...

class ExecuteResult(_message.Message):
    __slots__ = ("work_id", "request_id", "output_text", "done", "finish_reason", "computed_tokens", "generated_tokens", "execution_ms", "error_message")
    WORK_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TEXT_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    FINISH_REASON_FIELD_NUMBER: _ClassVar[int]
    COMPUTED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    GENERATED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    EXECUTION_MS_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    work_id: str
    request_id: str
    output_text: str
    done: bool
    finish_reason: _core_pb2.FinishReason
    computed_tokens: int
    generated_tokens: int
    execution_ms: int
    error_message: str
    def __init__(self, work_id: _Optional[str] = ..., request_id: _Optional[str] = ..., output_text: _Optional[str] = ..., done: _Optional[bool] = ..., finish_reason: _Optional[_Union[_core_pb2.FinishReason, str]] = ..., computed_tokens: _Optional[int] = ..., generated_tokens: _Optional[int] = ..., execution_ms: _Optional[int] = ..., error_message: _Optional[str] = ...) -> None: ...
