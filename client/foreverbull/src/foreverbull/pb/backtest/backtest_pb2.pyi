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

class Position(_message.Message):
    __slots__ = ("symbol", "amount", "cost_basis", "last_sale_price", "last_sale_date")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    AMOUNT_FIELD_NUMBER: _ClassVar[int]
    COST_BASIS_FIELD_NUMBER: _ClassVar[int]
    LAST_SALE_PRICE_FIELD_NUMBER: _ClassVar[int]
    LAST_SALE_DATE_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    amount: int
    cost_basis: float
    last_sale_price: float
    last_sale_date: _timestamp_pb2.Timestamp
    def __init__(
        self,
        symbol: _Optional[str] = ...,
        amount: _Optional[int] = ...,
        cost_basis: _Optional[float] = ...,
        last_sale_price: _Optional[float] = ...,
        last_sale_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
    ) -> None: ...

class Portfolio(_message.Message):
    __slots__ = (
        "timestamp",
        "cash_flow",
        "starting_cash",
        "portfolio_value",
        "pnl",
        "returns",
        "cash",
        "positions_value",
        "positions_exposure",
        "positions",
    )
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    CASH_FLOW_FIELD_NUMBER: _ClassVar[int]
    STARTING_CASH_FIELD_NUMBER: _ClassVar[int]
    PORTFOLIO_VALUE_FIELD_NUMBER: _ClassVar[int]
    PNL_FIELD_NUMBER: _ClassVar[int]
    RETURNS_FIELD_NUMBER: _ClassVar[int]
    CASH_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_VALUE_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_EXPOSURE_FIELD_NUMBER: _ClassVar[int]
    POSITIONS_FIELD_NUMBER: _ClassVar[int]
    timestamp: _timestamp_pb2.Timestamp
    cash_flow: float
    starting_cash: float
    portfolio_value: float
    pnl: float
    returns: float
    cash: float
    positions_value: float
    positions_exposure: float
    positions: _containers.RepeatedCompositeFieldContainer[Position]
    def __init__(
        self,
        timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...,
        cash_flow: _Optional[float] = ...,
        starting_cash: _Optional[float] = ...,
        portfolio_value: _Optional[float] = ...,
        pnl: _Optional[float] = ...,
        returns: _Optional[float] = ...,
        cash: _Optional[float] = ...,
        positions_value: _Optional[float] = ...,
        positions_exposure: _Optional[float] = ...,
        positions: _Optional[_Iterable[_Union[Position, _Mapping]]] = ...,
    ) -> None: ...

class Order(_message.Message):
    __slots__ = ("symbol", "amount")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    AMOUNT_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    amount: int
    def __init__(self, symbol: _Optional[str] = ..., amount: _Optional[int] = ...) -> None: ...

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

class IngestRequest(_message.Message):
    __slots__ = ("ingestion", "upload")
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_FIELD_NUMBER: _ClassVar[int]
    ingestion: Ingestion
    upload: bool
    def __init__(self, ingestion: _Optional[_Union[Ingestion, _Mapping]] = ..., upload: bool = ...) -> None: ...

class IngestResponse(_message.Message):
    __slots__ = ("ingestion",)
    INGESTION_FIELD_NUMBER: _ClassVar[int]
    ingestion: Ingestion
    def __init__(self, ingestion: _Optional[_Union[Ingestion, _Mapping]] = ...) -> None: ...

class RunRequest(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: Backtest
    def __init__(self, backtest: _Optional[_Union[Backtest, _Mapping]] = ...) -> None: ...

class RunResponse(_message.Message):
    __slots__ = ("backtest",)
    BACKTEST_FIELD_NUMBER: _ClassVar[int]
    backtest: Backtest
    def __init__(self, backtest: _Optional[_Union[Backtest, _Mapping]] = ...) -> None: ...

class PlaceOrdersRequest(_message.Message):
    __slots__ = ("orders",)
    ORDERS_FIELD_NUMBER: _ClassVar[int]
    orders: _containers.RepeatedCompositeFieldContainer[Order]
    def __init__(self, orders: _Optional[_Iterable[_Union[Order, _Mapping]]] = ...) -> None: ...

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
    portfolio: Portfolio
    def __init__(self, is_running: bool = ..., portfolio: _Optional[_Union[Portfolio, _Mapping]] = ...) -> None: ...

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
    periods: _containers.RepeatedCompositeFieldContainer[Period]
    def __init__(self, periods: _Optional[_Iterable[_Union[Period, _Mapping]]] = ...) -> None: ...

class StopRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
