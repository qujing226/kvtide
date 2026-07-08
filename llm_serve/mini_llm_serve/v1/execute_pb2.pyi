from mini_llm_serve.v1 import core_pb2 as _core_pb2
from mini_llm_serve.v1 import block_pb2 as _block_pb2
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
    __slots__ = ("work_id", "request_id", "phase", "token_ids", "computed_tokens", "generated_tokens", "num_new_tokens", "kv_blocks")
    WORK_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    PHASE_FIELD_NUMBER: _ClassVar[int]
    TOKEN_IDS_FIELD_NUMBER: _ClassVar[int]
    COMPUTED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    GENERATED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    NUM_NEW_TOKENS_FIELD_NUMBER: _ClassVar[int]
    KV_BLOCKS_FIELD_NUMBER: _ClassVar[int]
    work_id: str
    request_id: str
    phase: _core_pb2.WorkPhase
    token_ids: _containers.RepeatedScalarFieldContainer[int]
    computed_tokens: int
    generated_tokens: int
    num_new_tokens: int
    kv_blocks: _block_pb2.KVBlockMetadata
    def __init__(self, work_id: _Optional[str] = ..., request_id: _Optional[str] = ..., phase: _Optional[_Union[_core_pb2.WorkPhase, str]] = ..., token_ids: _Optional[_Iterable[int]] = ..., computed_tokens: _Optional[int] = ..., generated_tokens: _Optional[int] = ..., num_new_tokens: _Optional[int] = ..., kv_blocks: _Optional[_Union[_block_pb2.KVBlockMetadata, _Mapping]] = ...) -> None: ...

class ExecuteResult(_message.Message):
    __slots__ = ("work_id", "request_id", "token_id", "done", "finish_reason", "computed_tokens", "generated_tokens", "execution_ms", "error_message")
    WORK_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    TOKEN_ID_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    FINISH_REASON_FIELD_NUMBER: _ClassVar[int]
    COMPUTED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    GENERATED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    EXECUTION_MS_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    work_id: str
    request_id: str
    token_id: int
    done: bool
    finish_reason: _core_pb2.FinishReason
    computed_tokens: int
    generated_tokens: int
    execution_ms: int
    error_message: str
    def __init__(self, work_id: _Optional[str] = ..., request_id: _Optional[str] = ..., token_id: _Optional[int] = ..., done: _Optional[bool] = ..., finish_reason: _Optional[_Union[_core_pb2.FinishReason, str]] = ..., computed_tokens: _Optional[int] = ..., generated_tokens: _Optional[int] = ..., execution_ms: _Optional[int] = ..., error_message: _Optional[str] = ...) -> None: ...
