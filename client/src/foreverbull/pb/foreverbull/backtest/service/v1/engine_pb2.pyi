from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2 as _backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2 as _execution_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2 as _marketdata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DownloadIngestionRequest(_message.Message):
    __slots__ = ("bucket", "object")
    BUCKET_FIELD_NUMBER: _ClassVar[int]
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    bucket: str
    object: str
    def __init__(self, bucket: _Optional[str] = ..., object: _Optional[str] = ...) -> None: ...

class DownloadIngestionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class IngestRequest(_message.Message):
    __slots__ = ("backtest", "bucket", "object")
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    BUCKET_FIELD_NUMBER: _ClassVar[int]
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    bucket: str
    object: str
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ..., bucket: _Optional[str] = ..., object: _Optional[str] = ...) -> None: ...

class IngestResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class NewSessionRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class NewSessionResponse(_message.Message):
    __slots__ = ("port",)
    PORT_FIELD_NUMBER: _ClassVar[int]
    port: int
    def __init__(self, port: _Optional[int] = ...) -> None: ...

class RunBacktestRequest(_message.Message):
    __slots__ = ("execution", "benchmark")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    execution: _execution_pb2.Execution
    benchmark: str
    def __init__(self, execution: _Optional[_Union[_execution_pb2.Execution, _Mapping]] = ..., benchmark: _Optional[str] = ...) -> None: ...

class RunBacktestResponse(_message.Message):
    __slots__ = ("execution", "num_periods")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    NUM_PERIODS_FIELD_NUMBER: _ClassVar[int]
    execution: _execution_pb2.Execution
    num_periods: int
    def __init__(self, execution: _Optional[_Union[_execution_pb2.Execution, _Mapping]] = ..., num_periods: _Optional[int] = ...) -> None: ...

class GetCurrentPeriodRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCurrentPeriodResponse(_message.Message):
    __slots__ = ("is_running", "portfolio")
    IS_RUNNING_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    is_running: bool
    portfolio: _marketdata_pb2.Portfolio
    def __init__(self, is_running: bool = ..., portfolio: _Optional[_Union[_marketdata_pb2.Portfolio, _Mapping]] = ...) -> None: ...

class PlaceOrdersAndContinueRequest(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_marketdata_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_marketdata_pb2.Order, _Mapping]]] = ...) -> None: ...

class PlaceOrdersAndContinueResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetResultRequest(_message.Message):
    __slots__ = ("execution", "upload")
    class Upload(_message.Message):
        __slots__ = ("bucket", "object")
        BUCKET_FIELD_NUMBER: _ClassVar[int]
        OBJECT_FIELD_NUMBER: _ClassVar[int]
        bucket: str
        object: str
        def __init__(self, bucket: _Optional[str] = ..., object: _Optional[str] = ...) -> None: ...
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_FIELD_NUMBER: _ClassVar[int]
    execution: str
    upload: GetResultRequest.Upload
    def __init__(self, execution: _Optional[str] = ..., upload: _Optional[_Union[GetResultRequest.Upload, _Mapping]] = ...) -> None: ...

class GetResultResponse(_message.Message):
    __slots__ = ("periods",)
    PERIODS_FIELD_NUMBER: _ClassVar[int]
    periods: _containers.RepeatedCompositeFieldContainer[_execution_pb2.Period]
    def __init__(self, periods: _Optional[_Iterable[_Union[_execution_pb2.Period, _Mapping]]] = ...) -> None: ...
