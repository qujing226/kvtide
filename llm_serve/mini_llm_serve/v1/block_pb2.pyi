from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class KVBlockMetadata(_message.Message):
    __slots__ = ("block_size", "block_table", "allocated_blocks")
    BLOCK_SIZE_FIELD_NUMBER: _ClassVar[int]
    BLOCK_TABLE_FIELD_NUMBER: _ClassVar[int]
    ALLOCATED_BLOCKS_FIELD_NUMBER: _ClassVar[int]
    block_size: int
    block_table: _containers.RepeatedScalarFieldContainer[int]
    allocated_blocks: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, block_size: _Optional[int] = ..., block_table: _Optional[_Iterable[int]] = ..., allocated_blocks: _Optional[_Iterable[int]] = ...) -> None: ...
