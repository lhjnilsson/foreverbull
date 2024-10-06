from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Backtest(_message.Message):
    __slots__ = ("name", "start_date", "end_date", "symbols", "benchmark", "statuses")
    class Status(_message.Message):
        __slots__ = ("status", "error", "occurred_at")
        class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
            __slots__ = ()
            CREATED: _ClassVar[Backtest.Status.Status]
            READY: _ClassVar[Backtest.Status.Status]
            ERROR: _ClassVar[Backtest.Status.Status]
        CREATED: Backtest.Status.Status
        READY: Backtest.Status.Status
        ERROR: Backtest.Status.Status
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
        status: Backtest.Status.Status
        error: str
        occurred_at: _timestamp_pb2.Timestamp
        def __init__(self, status: _Optional[_Union[Backtest.Status.Status, str]] = ..., error: _Optional[str] = ..., occurred_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    name: str
    start_date: _common_pb2.Date
    end_date: _common_pb2.Date
    symbols: _containers.RepeatedScalarFieldContainer[str]
    benchmark: str
    statuses: _containers.RepeatedCompositeFieldContainer[Backtest.Status]
    def __init__(self, name: _Optional[str] = ..., start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., symbols: _Optional[_Iterable[str]] = ..., benchmark: _Optional[str] = ..., statuses: _Optional[_Iterable[_Union[Backtest.Status, _Mapping]]] = ...) -> None: ...
