from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Backtest(_message.Message):
    __slots__ = ("name", "start_date", "end_date", "symbols", "benchmark")
    NAME_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    name: str
    start_date: _common_pb2.Date
    end_date: _common_pb2.Date
    symbols: _containers.RepeatedScalarFieldContainer[str]
    benchmark: str
    def __init__(self, name: _Optional[str] = ..., start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., symbols: _Optional[_Iterable[str]] = ..., benchmark: _Optional[str] = ...) -> None: ...
