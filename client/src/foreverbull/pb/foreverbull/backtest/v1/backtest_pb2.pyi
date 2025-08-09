from foreverbull.pb.foreverbull import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Backtest(_message.Message):
    __slots__ = ("name", "benchmark", "periods")
    class Period(_message.Message):
        __slots__ = ("start_date", "end_date", "assets")
        class Asset(_message.Message):
            __slots__ = ("symbol", "enabled")
            SYMBOL_FIELD_NUMBER: _ClassVar[int]
            ENABLED_FIELD_NUMBER: _ClassVar[int]
            symbol: str
            enabled: bool
            def __init__(self, symbol: _Optional[str] = ..., enabled: bool = ...) -> None: ...
        START_DATE_FIELD_NUMBER: _ClassVar[int]
        END_DATE_FIELD_NUMBER: _ClassVar[int]
        ASSETS_FIELD_NUMBER: _ClassVar[int]
        start_date: _common_pb2.Date
        end_date: _common_pb2.Date
        assets: _containers.RepeatedCompositeFieldContainer[Backtest.Period.Asset]
        def __init__(self, start_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., end_date: _Optional[_Union[_common_pb2.Date, _Mapping]] = ..., assets: _Optional[_Iterable[_Union[Backtest.Period.Asset, _Mapping]]] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    PERIODS_FIELD_NUMBER: _ClassVar[int]
    name: str
    benchmark: str
    periods: _containers.RepeatedCompositeFieldContainer[Backtest.Period]
    def __init__(self, name: _Optional[str] = ..., benchmark: _Optional[str] = ..., periods: _Optional[_Iterable[_Union[Backtest.Period, _Mapping]]] = ...) -> None: ...
