import enum
from datetime import datetime, timezone
from typing import List, Optional

import pandas as pd
from pydantic import field_serializer

from foreverbull.entity.finance import Order, Portfolio

from .base import Base
from .service import Parameter


class BacktestStatusType(str, enum.Enum):
    CREATED = "CREATED"
    UPDATED = "UPDATED"
    INGESTING = "INGESTING"
    READY = "READY"
    ERROR = "ERROR"


class BacktestStatus(Base):
    status: BacktestStatusType
    error: str | None = None
    occurred_at: datetime


class Backtest(Base):
    name: str
    service: Optional[str] = None
    calendar: str = "XNYS"
    start: datetime
    end: datetime
    benchmark: str | None = None
    symbols: List[str]

    data_frequency: str = "daily"
    capital_base: int = 100_000

    statuses: List[BacktestStatus] | None = None

    sessions: int | None = None

    @field_serializer("start")
    def start_iso(self, start: datetime, _info):
        if start.tzinfo is None:
            start = start.replace(tzinfo=timezone.utc)
        return start.strftime("%Y-%m-%dT%H:%M:%SZ")

    @field_serializer("end")
    def end_iso(self, end: datetime, _info):
        if end.tzinfo is None:
            end = end.replace(tzinfo=timezone.utc)
        return end.strftime("%Y-%m-%dT%H:%M:%SZ")


class SessionStatusType(str, enum.Enum):
    CREATED = "CREATED"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class SessionStatus(Base):
    status: SessionStatusType
    error: str | None = None
    occurred_at: datetime


class Session(Base):
    id: Optional[str] = None
    backtest: str
    manual: bool = False
    executions: int

    statuses: List[SessionStatus] = []

    port: int | None = None


class ExecutionStatusType(str, enum.Enum):
    CREATED = "CREATED"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class ExecutionStatus(Base):
    status: ExecutionStatusType
    error: str | None = None
    occurred_at: datetime


class Execution(Base):
    id: Optional[str] = None
    calendar: str = "XNYS"
    start: Optional[datetime] = None
    end: Optional[datetime] = None
    benchmark: Optional[str] = None
    symbols: Optional[List[str]] = None
    capital_base: int = 100000
    database: Optional[str] = None
    parameters: Optional[List[Parameter]] = []

    statuses: List[ExecutionStatus] = []

    port: int | None = None

    @field_serializer("start")
    def start_iso(self, start: datetime, _info):
        if start is None:
            return None

        if start.tzinfo is None:
            start = start.replace(tzinfo=timezone.utc)
        return start.isoformat()

    @field_serializer("end")
    def end_iso(self, end: datetime, _info):
        if end is None:
            return None

        if end.tzinfo is None:
            end = end.replace(tzinfo=timezone.utc)
        return end.isoformat()


class IngestConfig(Base):
    calendar: Optional[str] = None
    start: Optional[datetime] = None
    end: Optional[datetime] = None
    symbols: List[str] = []


class Period(Base):
    timestamp: datetime = None
    portfolio: Optional[Portfolio] = None
    symbols: List[str] = None
    new_orders: List[Order] = None
    shorts_count: Optional[int] = None
    pnl: Optional[int] = None
    long_value: Optional[int] = None
    short_value: Optional[int] = None
    long_exposure: Optional[int] = None
    starting_exposure: Optional[int] = None
    short_exposure: Optional[int] = None
    capital_used: Optional[int] = None
    gross_leverage: Optional[int] = None
    net_leverage: Optional[int] = None
    ending_exposure: Optional[int] = None
    starting_value: Optional[int] = None
    ending_value: Optional[int] = None
    starting_cash: Optional[int] = None
    ending_cash: Optional[int] = None
    returns: Optional[int] = None
    portfolio_value: Optional[int] = None
    longs_count: Optional[int] = None
    algo_volatility: Optional[int] = None
    sharpe: Optional[int] = None
    alpha: Optional[int] = None
    beta: Optional[int] = None
    sortino: Optional[int] = None
    max_drawdown: Optional[int] = None
    max_leverage: Optional[int] = None
    excess_return: Optional[int] = None
    treasury_period_return: Optional[int] = None
    benchmark_period_return: Optional[int] = None
    benchmark_volatility: Optional[int] = None
    algorithm_period_return: Optional[int] = None

    @classmethod
    def from_backtest(cls, period):
        return Period(
            timestamp=period["period_open"].to_pydatetime().replace(tzinfo=timezone.utc),
            portfolio=None,
            symbols=[],
            new_orders=[],
            shorts_count=int(period["shorts_count"] * 100),
            pnl=int(period["pnl"] * 100),
            long_value=int(period["long_value"] * 100),
            short_value=int(period["short_value"] * 100),
            long_exposure=int(period["long_exposure"] * 100),
            starting_exposure=int(period["starting_exposure"] * 100),
            short_exposure=int(period["short_exposure"] * 100),
            capital_used=int(period["capital_used"] * 100),
            gross_leverage=int(period["gross_leverage"] * 100),
            net_leverage=int(period["net_leverage"] * 100),
            ending_exposure=int(period["ending_exposure"] * 100),
            starting_value=int(period["starting_value"] * 100),
            ending_value=int(period["ending_value"] * 100),
            starting_cash=int(period["starting_cash"] * 100),
            ending_cash=int(period["ending_cash"] * 100),
            returns=int(period["returns"] * 100),
            portfolio_value=int(period["portfolio_value"] * 100),
            longs_count=int(period["longs_count"] * 100),
            algo_volatility=None if pd.isnull(period["algo_volatility"]) else int(period["algo_volatility"] * 100),
            sharpe=None if pd.isnull(period["sharpe"]) else int(period["sharpe"] * 100),
            alpha=None if period["alpha"] is None or pd.isnull(period["alpha"]) else int(period["alpha"] * 100),
            beta=None if period["beta"] is None or pd.isnull(period["beta"]) else int(period["beta"] * 100),
            sortino=None if pd.isnull(period["sortino"]) else int(period["sortino"] * 100),
            max_drawdown=None if pd.isnull(period["max_drawdown"]) else int(period["max_drawdown"] * 100),
            max_leverage=None if pd.isnull(period["max_leverage"]) else int(period["max_leverage"] * 100),
            excess_return=None if pd.isnull(period["excess_return"]) else int(period["excess_return"] * 100),
            treasury_period_return=(
                None if pd.isnull(period["treasury_period_return"]) else int(period["treasury_period_return"] * 100)
            ),
            benchmark_period_return=(
                None if pd.isnull(period["benchmark_period_return"]) else int(period["benchmark_period_return"] * 100)
            ),
            benchmark_volatility=(
                None if pd.isnull(period["benchmark_volatility"]) else int(period["benchmark_volatility"] * 100)
            ),
            algorithm_period_return=(
                None if pd.isnull(period["algorithm_period_return"]) else int(period["algorithm_period_return"] * 100)
            ),
        )


class Result(Base):
    periods: List[Period]
