from typing import ClassVar as _ClassVar
from typing import Optional as _Optional

from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message

DESCRIPTOR: _descriptor.FileDescriptor

class Request(_message.Message):
    __slots__ = ("task", "data")
    TASK_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    task: str
    data: bytes
    def __init__(self, task: _Optional[str] = ..., data: _Optional[bytes] = ...) -> None: ...

class Response(_message.Message):
    __slots__ = ("task", "data", "error")
    TASK_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    task: str
    data: bytes
    error: str
    def __init__(
        self, task: _Optional[str] = ..., data: _Optional[bytes] = ..., error: _Optional[str] = ...
    ) -> None: ...
