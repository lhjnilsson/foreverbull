from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Session(_message.Message):
    __slots__ = ("id", "backtest", "statuses", "executions", "port")
    class Status(_message.Message):
        __slots__ = ("status", "error", "occurred_at")
        class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
            __slots__ = ()
            CREATED: _ClassVar[Session.Status.Status]
            RUNNING: _ClassVar[Session.Status.Status]
            COMPLETED: _ClassVar[Session.Status.Status]
            FAILED: _ClassVar[Session.Status.Status]
        CREATED: Session.Status.Status
        RUNNING: Session.Status.Status
        COMPLETED: Session.Status.Status
        FAILED: Session.Status.Status
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
        status: Session.Status.Status
        error: str
        occurred_at: _timestamp_pb2.Timestamp
        def __init__(self, status: _Optional[_Union[Session.Status.Status, str]] = ..., error: _Optional[str] = ..., occurred_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    EXECUTIONS_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    id: str
    backtest: str
    statuses: _containers.RepeatedCompositeFieldContainer[Session.Status]
    executions: int
    port: int
    def __init__(self, id: _Optional[str] = ..., backtest: _Optional[str] = ..., statuses: _Optional[_Iterable[_Union[Session.Status, _Mapping]]] = ..., executions: _Optional[int] = ..., port: _Optional[int] = ...) -> None: ...
