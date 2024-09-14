from google.protobuf import timestamp_pb2 as _timestamp_pb2
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

class Ingestion(_message.Message):
    __slots__ = ("start_date", "end_date", "symbols")
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    start_date: _timestamp_pb2.Timestamp
    end_date: _timestamp_pb2.Timestamp
    symbols: _containers.RepeatedScalarFieldContainer[str]
    def __init__(
        self,
        start_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        end_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        symbols: _Optional[_Iterable[str]] = ...,
    ) -> None: ...

class Backtest(_message.Message):
    __slots__ = ("start_date", "end_date", "symbols", "benchmark")
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    start_date: _timestamp_pb2.Timestamp
    end_date: _timestamp_pb2.Timestamp
    symbols: _containers.RepeatedScalarFieldContainer[str]
    benchmark: str
    def __init__(
        self,
        start_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        end_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        symbols: _Optional[_Iterable[str]] = ...,
        benchmark: _Optional[str] = ...,
    ) -> None: ...

class Session(_message.Message):
    __slots__ = ("id", "manual", "backtest", "statuses", "executions", "port")

    class Status(_message.Message):
        __slots__ = ("status", "error", "occurred_at")
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
        status: str
        error: str
        occurred_at: _timestamp_pb2.Timestamp
        def __init__(
            self,
            status: _Optional[str] = ...,
            error: _Optional[str] = ...,
            occurred_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        ) -> None: ...

    ID_FIELD_NUMBER: _ClassVar[int]
    MANUAL_FIELD_NUMBER: _ClassVar[int]
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    EXECUTIONS_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    id: str
    manual: bool
    backtest: str
    statuses: _containers.RepeatedCompositeFieldContainer[Session.Status]
    executions: int
    port: int
    def __init__(
        self,
        id: _Optional[str] = ...,
        manual: bool = ...,
        backtest: _Optional[str] = ...,
        statuses: _Optional[_Iterable[_Union[Session.Status, _Mapping]]] = ...,
        executions: _Optional[int] = ...,
        port: _Optional[int] = ...,
    ) -> None: ...

class Execution(_message.Message):
    __slots__ = ("id", "session", "start_date", "end_date", "benchmark", "symbols", "statuses")

    class Status(_message.Message):
        __slots__ = ("status", "error", "occurred_at")
        STATUS_FIELD_NUMBER: _ClassVar[int]
        ERROR_FIELD_NUMBER: _ClassVar[int]
        OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
        status: str
        error: str
        occurred_at: _timestamp_pb2.Timestamp
        def __init__(
            self,
            status: _Optional[str] = ...,
            error: _Optional[str] = ...,
            occurred_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        ) -> None: ...

    ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_FIELD_NUMBER: _ClassVar[int]
    SYMBOLS_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    id: str
    session: str
    start_date: _timestamp_pb2.Timestamp
    end_date: _timestamp_pb2.Timestamp
    benchmark: str
    symbols: _containers.RepeatedScalarFieldContainer[str]
    statuses: _containers.RepeatedCompositeFieldContainer[Execution.Status]
    def __init__(
        self,
        id: _Optional[str] = ...,
        session: _Optional[str] = ...,
        start_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        end_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        benchmark: _Optional[str] = ...,
        symbols: _Optional[_Iterable[str]] = ...,
        statuses: _Optional[_Iterable[_Union[Execution.Status, _Mapping]]] = ...,
    ) -> None: ...

class Period(_message.Message):
    __slots__ = (
        "timestamp",
        "PNL",
        "returns",
        "portfolio_value",
        "longs_count",
        "shorts_count",
        "long_value",
        "short_value",
        "starting_exposure",
        "ending_exposure",
        "long_exposure",
        "short_exposure",
        "capital_used",
        "gross_leverage",
        "net_leverage",
        "starting_value",
        "ending_value",
        "starting_cash",
        "ending_cash",
        "max_drawdown",
        "max_leverage",
        "excess_return",
        "treasury_period_return",
        "algorithm_period_return",
        "algo_volatility",
        "sharpe",
        "sortino",
        "benchmark_period_return",
        "benchmark_volatility",
        "alpha",
        "beta",
        "positions",
    )
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    PNL_FIELD_NUMBER: _ClassVar[int]
    RETURNS_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_VALUE_FIELD_NUMBER: _ClassVar[int]
    LONGS_COUNT_FIELD_NUMBER: _ClassVar[int]
    SHORTS_COUNT_FIELD_NUMBER: _ClassVar[int]
    LONG_VALUE_FIELD_NUMBER: _ClassVar[int]
    SHORT_VALUE_FIELD_NUMBER: _ClassVar[int]
    STARTING_EXPOSURE_FIELD_NUMBER: _ClassVar[int]
    ENDING_EXPOSURE_FIELD_NUMBER: _ClassVar[int]
    LONG_EXPOSURE_FIELD_NUMBER: _ClassVar[int]
    SHORT_EXPOSURE_FIELD_NUMBER: _ClassVar[int]
    CAPITAL_USED_FIELD_NUMBER: _ClassVar[int]
    GROSS_LEVERAGE_FIELD_NUMBER: _ClassVar[int]
    NET_LEVERAGE_FIELD_NUMBER: _ClassVar[int]
    STARTING_VALUE_FIELD_NUMBER: _ClassVar[int]
    ENDING_VALUE_FIELD_NUMBER: _ClassVar[int]
    STARTING_CASH_FIELD_NUMBER: _ClassVar[int]
    ENDING_CASH_FIELD_NUMBER: _ClassVar[int]
    MAX_DRAWDOWN_FIELD_NUMBER: _ClassVar[int]
    MAX_LEVERAGE_FIELD_NUMBER: _ClassVar[int]
    EXCESS_RETURN_FIELD_NUMBER: _ClassVar[int]
    TREASURY_PERIOD_RETURN_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_PERIOD_RETURN_FIELD_NUMBER: _ClassVar[int]
    ALGO_VOLATILITY_FIELD_NUMBER: _ClassVar[int]
    SHARPE_FIELD_NUMBER: _ClassVar[int]
    SORTINO_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_PERIOD_RETURN_FIELD_NUMBER: _ClassVar[int]
    BENCHMARK_VOLATILITY_FIELD_NUMBER: _ClassVar[int]
    ALPHA_FIELD_NUMBER: _ClassVar[int]
    BETA_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_FIELD_NUMBER: _ClassVar[int]
    timestamp: _timestamp_pb2.Timestamp
    PNL: float
    returns: float
    portfolio_value: float
    longs_count: int
    shorts_count: int
    long_value: float
    short_value: float
    starting_exposure: float
    ending_exposure: float
    long_exposure: float
    short_exposure: float
    capital_used: float
    gross_leverage: float
    net_leverage: float
    starting_value: float
    ending_value: float
    starting_cash: float
    ending_cash: float
    max_drawdown: float
    max_leverage: float
    excess_return: float
    treasury_period_return: float
    algorithm_period_return: float
    algo_volatility: float
    sharpe: float
    sortino: float
    benchmark_period_return: float
    benchmark_volatility: float
    alpha: float
    beta: float
    positions: _containers.RepeatedCompositeFieldContainer[_finance_pb2.Position]
    def __init__(
        self,
        timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        PNL: _Optional[float] = ...,
        returns: _Optional[float] = ...,
        portfolio_value: _Optional[float] = ...,
        longs_count: _Optional[int] = ...,
        shorts_count: _Optional[int] = ...,
        long_value: _Optional[float] = ...,
        short_value: _Optional[float] = ...,
        starting_exposure: _Optional[float] = ...,
        ending_exposure: _Optional[float] = ...,
        long_exposure: _Optional[float] = ...,
        short_exposure: _Optional[float] = ...,
        capital_used: _Optional[float] = ...,
        gross_leverage: _Optional[float] = ...,
        net_leverage: _Optional[float] = ...,
        starting_value: _Optional[float] = ...,
        ending_value: _Optional[float] = ...,
        starting_cash: _Optional[float] = ...,
        ending_cash: _Optional[float] = ...,
        max_drawdown: _Optional[float] = ...,
        max_leverage: _Optional[float] = ...,
        excess_return: _Optional[float] = ...,
        treasury_period_return: _Optional[float] = ...,
        algorithm_period_return: _Optional[float] = ...,
        algo_volatility: _Optional[float] = ...,
        sharpe: _Optional[float] = ...,
        sortino: _Optional[float] = ...,
        benchmark_period_return: _Optional[float] = ...,
        benchmark_volatility: _Optional[float] = ...,
        alpha: _Optional[float] = ...,
        beta: _Optional[float] = ...,
        positions: _Optional[_Iterable[_Union[_finance_pb2.Position, _Mapping]]] = ...,
    ) -> None: ...
