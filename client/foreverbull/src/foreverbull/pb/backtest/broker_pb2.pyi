from foreverbull.pb.backtest import backtest_pb2 as _backtest_pb2
from foreverbull.pb.service import service_pb2 as _service_pb2
from foreverbull.pb.finance import finance_pb2 as _finance_pb2
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

class GetBacktestRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetBacktestResponse(_message.Message):
    __slots__ = ("name", "backtest")
    NAME_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    name: str
    backtest: _backtest_pb2.Backtest
    def __init__(
        self, name: _Optional[str] = ..., backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...
    ) -> None: ...

class CreateSessionRequest(_message.Message):
    __slots__ = ("backtest_name",)
    BACKTEST_NAME_FIELD_NUMBER: _ClassVar[int]
    backtest_name: str
    def __init__(self, backtest_name: _Optional[str] = ...) -> None: ...

class CreateSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: _backtest_pb2.Session
    def __init__(self, session: _Optional[_Union[_backtest_pb2.Session, _Mapping]] = ...) -> None: ...

class GetSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class GetSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: _backtest_pb2.Session
    def __init__(self, session: _Optional[_Union[_backtest_pb2.Session, _Mapping]] = ...) -> None: ...

class CreateExecutionRequest(_message.Message):
    __slots__ = ("session_id", "backtest", "benchmark")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    backtest: _backtest_pb2.Backtest
    benchmark: str
    def __init__(
        self,
        session_id: _Optional[str] = ...,
        backtest: _Optional[_Union[_backtest_pb2.Backtest, _Mapping]] = ...,
        benchmark: _Optional[str] = ...,
    ) -> None: ...

class CreateExecutionResponse(_message.Message):
    __slots__ = ("configuration",)
    CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    configuration: _service_pb2.ExecutionConfiguration
    def __init__(
        self, configuration: _Optional[_Union[_service_pb2.ExecutionConfiguration, _Mapping]] = ...
    ) -> None: ...

class RunExecutionRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class RunExecutionResponse(_message.Message):
    __slots__ = ("execution", "portfolio")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_FIELD_NUMBER: _ClassVar[int]
    execution: _backtest_pb2.Execution
    portfolio: _finance_pb2.Portfolio
    def __init__(
        self,
        execution: _Optional[_Union[_backtest_pb2.Execution, _Mapping]] = ...,
        portfolio: _Optional[_Union[_finance_pb2.Portfolio, _Mapping]] = ...,
    ) -> None: ...

class GetExecutionRequest(_message.Message):
    __slots__ = ("execution_id",)
    EXECUTION_ID_FIELD_NUMBER: _ClassVar[int]
    execution_id: str
    def __init__(self, execution_id: _Optional[str] = ...) -> None: ...

class GetExecutionResponse(_message.Message):
    __slots__ = ("execution", "periods")
    EXECUTION_FIELD_NUMBER: _ClassVar[int]
    PERIODS_FIELD_NUMBER: _ClassVar[int]
    execution: _backtest_pb2.Execution
    periods: _containers.RepeatedCompositeFieldContainer[_backtest_pb2.Period]
    def __init__(
        self,
        execution: _Optional[_Union[_backtest_pb2.Execution, _Mapping]] = ...,
        periods: _Optional[_Iterable[_Union[_backtest_pb2.Period, _Mapping]]] = ...,
    ) -> None: ...
