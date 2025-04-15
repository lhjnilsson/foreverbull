from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class SessionStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CREATED: _ClassVar[SessionStatus]
    RUNNING: _ClassVar[SessionStatus]
    COMPLETED: _ClassVar[SessionStatus]
    FAILED: _ClassVar[SessionStatus]
CREATED: SessionStatus
RUNNING: SessionStatus
COMPLETED: SessionStatus
FAILED: SessionStatus

class Session(_message.Message):
    __slots__ = ("id", "backtest", "port")
    ID_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    id: str
    backtest: str
    port: int
    def __init__(self, id: _Optional[str] = ..., backtest: _Optional[str] = ..., port: _Optional[int] = ...) -> None: ...
