from foreverbull.pb.backtest import backtest_pb2 as _backtest_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import (
    ClassVar as _ClassVar,
    Iterable as _Iterable,
    Mapping as _Mapping,
    Optional as _Optional,
    Union as _Union,
)

DESCRIPTOR: _descriptor.FileDescriptor

class IngestRequest(_message.Message):
    __slots__ = ("ingestion", "upload")
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_FIELD_NUMBER: _ClassVar[int]
    ingestion: _backtest_pb2.Ingestion
    upload: bool
    def __init__(
        self, ingestion: _Optional[_Union[_backtest_pb2.Ingestion, _Mapping]] = ..., upload: bool = ...
    ) -> None: ...

class IngestResponse(_message.Message):
    __slots__ = ("ingestion",)
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    ingestion: _backtest_pb2.Ingestion
    def __init__(self, ingestion: _Optional[_Union[_backtest_pb2.Ingestion, _Mapping]] = ...) -> None: ...

class RunRequest(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class RunResponse(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class PlaceOrdersRequest(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_backtest_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_backtest_pb2.Order, _Mapping]]] = ...) -> None: ...

class PlaceOrdersResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetNextPeriodRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetNextPeriodResponse(_message.Message):
    __slots__ = ("is_running", "portfolio")
    IS_RUNNING_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    is_running: bool
    portfolio: _backtest_pb2.Portfolio
    def __init__(
        self, is_running: bool = ..., portfolio: _Optional[_Union[_backtest_pb2.Portfolio, _Mapping]] = ...
    ) -> None: ...

class GetResultRequest(_message.Message):
    __slots__ = ("execution", "upload")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_FIELD_NUMBER: _ClassVar[int]
    execution: str
    upload: bool
    def __init__(self, execution: _Optional[str] = ..., upload: bool = ...) -> None: ...

class GetResultResponse(_message.Message):
    __slots__ = ("periods",)
    PERIODS_FIELD_NUMBER: _ClassVar[int]
    periods: _containers.RepeatedCompositeFieldContainer[_backtest_pb2.Period]
    def __init__(self, periods: _Optional[_Iterable[_Union[_backtest_pb2.Period, _Mapping]]] = ...) -> None: ...

class StopRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
