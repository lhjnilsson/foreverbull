from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class State(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    STATE_UNSPECIFIED: _ClassVar[State]
    STATE_RUNNING: _ClassVar[State]
    STATE_COMPLETED: _ClassVar[State]
    STATE_FAILED: _ClassVar[State]
STATE_UNSPECIFIED: State
STATE_RUNNING: State
STATE_COMPLETED: State
STATE_FAILED: State

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
    def __init__(self, task: _Optional[str] = ..., data: _Optional[bytes] = ..., error: _Optional[str] = ...) -> None: ...

class Date(_message.Message):
    __slots__ = ("year", "month", "day")
    YEAR_FIELD_NUMBER: _ClassVar[int]
    MONTH_FIELD_NUMBER: _ClassVar[int]
    DAY_FIELD_NUMBER: _ClassVar[int]
    year: int
    month: int
    day: int
    def __init__(self, year: _Optional[int] = ..., month: _Optional[int] = ..., day: _Optional[int] = ...) -> None: ...
