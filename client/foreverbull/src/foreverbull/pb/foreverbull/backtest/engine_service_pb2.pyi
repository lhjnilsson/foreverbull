from foreverbull.pb.foreverbull.backtest import backtest_pb2 as _backtest_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2 as _finance_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2 as _execution_pb2
from foreverbull.pb.foreverbull.backtest import ingestion_pb2 as _ingestion_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetIngestionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetIngestionResponse(_message.Message):
    __slots__ = ("ingestion",)
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    ingestion: _ingestion_pb2.Ingestion
    def __init__(self, ingestion: _Optional[_Union[_ingestion_pb2.Ingestion, _Mapping]] = ...) -> None: ...

class DownloadIngestionRequest(_message.Message):
    __slots__ = ("bucket", "object")
    BUCKET_FIELD_NUMBER: _ClassVar[int]
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    bucket: str
    object: str
    def __init__(self, bucket: _Optional[str] = ..., object: _Optional[str] = ...) -> None: ...

class DownloadIngestionResponse(_message.Message):
    __slots__ = ("ingestion",)
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    ingestion: _ingestion_pb2.Ingestion
    def __init__(self, ingestion: _Optional[_Union[_ingestion_pb2.Ingestion, _Mapping]] = ...) -> None: ...

class IngestRequest(_message.Message):
    __slots__ = ("ingestion", "bucket", "object")
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    BUCKET_FIELD_NUMBER: _ClassVar[int]
    OBJECT_FIELD_NUMBER: _ClassVar[int]
    ingestion: _ingestion_pb2.Ingestion
    bucket: str
    object: str
    def __init__(self, ingestion: _Optional[_Union[_ingestion_pb2.Ingestion, _Mapping]] = ..., bucket: _Optional[str] = ..., object: _Optional[str] = ...) -> None: ...

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
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class RunBacktestResponse(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: _backtest_pb2.Backtest
    def __init__(self, backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...) -> None: ...

class GetCurrentPeriodRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCurrentPeriodResponse(_message.Message):
    __slots__ = ("is_running", "portfolio")
    IS_RUNNING_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    is_running: bool
    portfolio: _finance_pb2.Portfolio
    def __init__(self, is_running: bool = ..., portfolio: _Optional[_Union[_finance_pb2.Portfolio, _Mapping]] = ...) -> None: ...

class PlaceOrdersAndContinueRequest(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[_finance_pb2.Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[_finance_pb2.Order, _Mapping]]] = ...) -> None: ...

class PlaceOrdersAndContinueResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

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
    periods: _containers.RepeatedCompositeFieldContainer[_execution_pb2.Period]
    def __init__(self, periods: _Optional[_Iterable[_Union[_execution_pb2.Period, _Mapping]]] = ...) -> None: ...
