from kvtide.v1 import core_pb2 as _core_pb2
from kvtide.v1 import block_pb2 as _block_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetRuntimeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetRuntimeResponse(_message.Message):
    __slots__ = ("executor_id", "runtime_epoch", "model_id", "model_type", "dtype", "device_type", "tensor_parallel_size", "block_size", "num_kv_blocks", "num_hidden_layers", "num_kv_heads", "head_dim", "total_memory_bytes", "available_memory_bytes", "kv_cache_bytes", "model_revision", "kv_layout_version", "kv_compatibility_id", "transfer_endpoint")
    EXECUTOR_ID_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_EPOCH_FIELD_NUMBER: _ClassVar[int]
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_TYPE_FIELD_NUMBER: _ClassVar[int]
    DTYPE_FIELD_NUMBER: _ClassVar[int]
    DEVICE_TYPE_FIELD_NUMBER: _ClassVar[int]
    TENSOR_PARALLEL_SIZE_FIELD_NUMBER: _ClassVar[int]
    BLOCK_SIZE_FIELD_NUMBER: _ClassVar[int]
    NUM_KV_BLOCKS_FIELD_NUMBER: _ClassVar[int]
    NUM_HIDDEN_LAYERS_FIELD_NUMBER: _ClassVar[int]
    NUM_KV_HEADS_FIELD_NUMBER: _ClassVar[int]
    HEAD_DIM_FIELD_NUMBER: _ClassVar[int]
    TOTAL_MEMORY_BYTES_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_MEMORY_BYTES_FIELD_NUMBER: _ClassVar[int]
    KV_CACHE_BYTES_FIELD_NUMBER: _ClassVar[int]
    MODEL_REVISION_FIELD_NUMBER: _ClassVar[int]
    KV_LAYOUT_VERSION_FIELD_NUMBER: _ClassVar[int]
    KV_COMPATIBILITY_ID_FIELD_NUMBER: _ClassVar[int]
    TRANSFER_ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    executor_id: str
    runtime_epoch: int
    model_id: str
    model_type: str
    dtype: str
    device_type: str
    tensor_parallel_size: int
    block_size: int
    num_kv_blocks: int
    num_hidden_layers: int
    num_kv_heads: int
    head_dim: int
    total_memory_bytes: int
    available_memory_bytes: int
    kv_cache_bytes: int
    model_revision: str
    kv_layout_version: int
    kv_compatibility_id: str
    transfer_endpoint: str
    def __init__(self, executor_id: _Optional[str] = ..., runtime_epoch: _Optional[int] = ..., model_id: _Optional[str] = ..., model_type: _Optional[str] = ..., dtype: _Optional[str] = ..., device_type: _Optional[str] = ..., tensor_parallel_size: _Optional[int] = ..., block_size: _Optional[int] = ..., num_kv_blocks: _Optional[int] = ..., num_hidden_layers: _Optional[int] = ..., num_kv_heads: _Optional[int] = ..., head_dim: _Optional[int] = ..., total_memory_bytes: _Optional[int] = ..., available_memory_bytes: _Optional[int] = ..., kv_cache_bytes: _Optional[int] = ..., model_revision: _Optional[str] = ..., kv_layout_version: _Optional[int] = ..., kv_compatibility_id: _Optional[str] = ..., transfer_endpoint: _Optional[str] = ...) -> None: ...

class ExecuteBatchRequest(_message.Message):
    __slots__ = ("batch_id", "runtime_epoch", "items")
    BATCH_ID_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_EPOCH_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    batch_id: str
    runtime_epoch: int
    items: _containers.RepeatedCompositeFieldContainer[ExecuteItem]
    def __init__(self, batch_id: _Optional[str] = ..., runtime_epoch: _Optional[int] = ..., items: _Optional[_Iterable[_Union[ExecuteItem, _Mapping]]] = ...) -> None: ...

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
    __slots__ = ("work_id", "request_id", "phase", "token_ids", "computed_tokens", "generated_tokens", "num_new_tokens", "kv_blocks", "sample")
    WORK_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    PHASE_FIELD_NUMBER: _ClassVar[int]
    TOKEN_IDS_FIELD_NUMBER: _ClassVar[int]
    COMPUTED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    GENERATED_TOKENS_FIELD_NUMBER: _ClassVar[int]
    NUM_NEW_TOKENS_FIELD_NUMBER: _ClassVar[int]
    KV_BLOCKS_FIELD_NUMBER: _ClassVar[int]
    SAMPLE_FIELD_NUMBER: _ClassVar[int]
    work_id: str
    request_id: str
    phase: _core_pb2.WorkPhase
    token_ids: _containers.RepeatedScalarFieldContainer[int]
    computed_tokens: int
    generated_tokens: int
    num_new_tokens: int
    kv_blocks: _block_pb2.KVBlockMetadata
    sample: bool
    def __init__(self, work_id: _Optional[str] = ..., request_id: _Optional[str] = ..., phase: _Optional[_Union[_core_pb2.WorkPhase, str]] = ..., token_ids: _Optional[_Iterable[int]] = ..., computed_tokens: _Optional[int] = ..., generated_tokens: _Optional[int] = ..., num_new_tokens: _Optional[int] = ..., kv_blocks: _Optional[_Union[_block_pb2.KVBlockMetadata, _Mapping]] = ..., sample: _Optional[bool] = ...) -> None: ...

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

class ReleaseBlocksRequest(_message.Message):
    __slots__ = ("runtime_epoch", "block_ids")
    RUNTIME_EPOCH_FIELD_NUMBER: _ClassVar[int]
    BLOCK_IDS_FIELD_NUMBER: _ClassVar[int]
    runtime_epoch: int
    block_ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, runtime_epoch: _Optional[int] = ..., block_ids: _Optional[_Iterable[int]] = ...) -> None: ...

class ReleaseBlocksResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PushKVRequest(_message.Message):
    __slots__ = ("transfer_id", "source_executor_id", "source_runtime_epoch", "destination_executor_id", "destination_runtime_epoch", "kv_compatibility_id", "prefix_hash", "prefix_tokens", "destination_block_ids", "key_data", "value_data")
    TRANSFER_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_EXECUTOR_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_RUNTIME_EPOCH_FIELD_NUMBER: _ClassVar[int]
    DESTINATION_EXECUTOR_ID_FIELD_NUMBER: _ClassVar[int]
    DESTINATION_RUNTIME_EPOCH_FIELD_NUMBER: _ClassVar[int]
    KV_COMPATIBILITY_ID_FIELD_NUMBER: _ClassVar[int]
    PREFIX_HASH_FIELD_NUMBER: _ClassVar[int]
    PREFIX_TOKENS_FIELD_NUMBER: _ClassVar[int]
    DESTINATION_BLOCK_IDS_FIELD_NUMBER: _ClassVar[int]
    KEY_DATA_FIELD_NUMBER: _ClassVar[int]
    VALUE_DATA_FIELD_NUMBER: _ClassVar[int]
    transfer_id: str
    source_executor_id: str
    source_runtime_epoch: int
    destination_executor_id: str
    destination_runtime_epoch: int
    kv_compatibility_id: str
    prefix_hash: str
    prefix_tokens: int
    destination_block_ids: _containers.RepeatedScalarFieldContainer[int]
    key_data: bytes
    value_data: bytes
    def __init__(self, transfer_id: _Optional[str] = ..., source_executor_id: _Optional[str] = ..., source_runtime_epoch: _Optional[int] = ..., destination_executor_id: _Optional[str] = ..., destination_runtime_epoch: _Optional[int] = ..., kv_compatibility_id: _Optional[str] = ..., prefix_hash: _Optional[str] = ..., prefix_tokens: _Optional[int] = ..., destination_block_ids: _Optional[_Iterable[int]] = ..., key_data: _Optional[bytes] = ..., value_data: _Optional[bytes] = ...) -> None: ...

class PushKVResponse(_message.Message):
    __slots__ = ("transfer_id", "destination_executor_id", "destination_runtime_epoch")
    TRANSFER_ID_FIELD_NUMBER: _ClassVar[int]
    DESTINATION_EXECUTOR_ID_FIELD_NUMBER: _ClassVar[int]
    DESTINATION_RUNTIME_EPOCH_FIELD_NUMBER: _ClassVar[int]
    transfer_id: str
    destination_executor_id: str
    destination_runtime_epoch: int
    def __init__(self, transfer_id: _Optional[str] = ..., destination_executor_id: _Optional[str] = ..., destination_runtime_epoch: _Optional[int] = ...) -> None: ...
