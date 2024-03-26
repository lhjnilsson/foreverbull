import enum
from datetime import datetime, timezone
from typing import List, Optional

import pandas as pd

from foreverbull.entity.base import Base


class Asset(Base):
    symbol: str


class IngestConfig(Base):
    calendar: Optional[str] = None
    start: Optional[datetime] = None
    end: Optional[datetime] = None
    symbols: List[str] = []


class Position(Base):
    symbol: str
    amount: int
    cost_basis: float
    last_sale_price: float
    last_sale_date: datetime


class OrderStatus(enum.IntEnum):
    OPEN = 0
    FILLED = 1
    CANCELLED = 2
    REJECTED = 3
    HELD = 4


class Order(Base):
    id: Optional[str] = None
    symbol: str
    amount: int
    filled: Optional[int] = None
    commission: Optional[float] = None
    stop_price: Optional[float] = None
    limit_price: Optional[float] = None
    stop_reached: bool = False
    limit_reached: bool = False

    created_at: Optional[datetime] = None
    current_timestamp: Optional[datetime] = None

    status: Optional[OrderStatus] = None

    @classmethod
    def from_zipline(cls, order):
        return cls(
            id=order.id,
            symbol=order.sid.symbol,
            amount=order.amount,
            filled=order.filled,
            commission=order.commission,
            stop_price=order.stop,
            limit_price=order.limit,
            stop_reached=order.stop_reached,
            limit_reached=order.limit_reached,
            created_at=order.created,
            current_timestamp=order.dt,
            status=order.status,
        )


class BasePeriod(Base):
    timestamp: datetime


class RunningPeriod(BasePeriod):
    cash_flow: float
    starting_cash: float
    portfolio_value: float
    pnl: float  # profit and loss
    returns: float
    cash: float
    positions_value: float
    positions_exposure: float

    positions: List[Position]
    new_orders: List[Order]

    @classmethod
    def from_zipline(cls, trading_algorithm, new_orders):
        return RunningPeriod(
            timestamp=trading_algorithm.datetime,
            cash_flow=trading_algorithm.portfolio.cash_flow,
            starting_cash=trading_algorithm.portfolio.starting_cash,
            portfolio_value=trading_algorithm.portfolio.portfolio_value,
            pnl=trading_algorithm.portfolio.pnl,
            returns=trading_algorithm.portfolio.returns,
            cash=trading_algorithm.portfolio.cash,
            positions_value=trading_algorithm.portfolio.positions_value,
            positions_exposure=trading_algorithm.portfolio.positions_exposure,
            positions=[
                Position(
                    symbol=position.sid.symbol,
                    amount=position.amount,
                    cost_basis=position.cost_basis,
                    last_sale_price=position.last_sale_price,
                    last_sale_date=position.last_sale_date,
                )
                for _, position in trading_algorithm.portfolio.positions.items()
            ],
            new_orders=[Order.from_zipline(order) for order in new_orders],
        )


class ResultPeriod(BasePeriod):
    shorts_count: int
    pnl: int
    long_value: int
    short_value: int
    long_exposure: int
    starting_exposure: int
    short_exposure: int
    capital_used: int
    gross_leverage: int
    net_leverage: int
    ending_exposure: int
    starting_value: int
    ending_value: int
    starting_cash: int
    ending_cash: int
    returns: int
    portfolio_value: int
    longs_count: int
    algo_volatility: Optional[int]
    sharpe: Optional[int]
    alpha: Optional[int]
    beta: Optional[int]
    sortino: Optional[int]
    max_drawdown: Optional[int]
    max_leverage: Optional[int]
    excess_return: Optional[int]
    treasury_period_return: Optional[int]
    benchmark_period_return: Optional[int]
    benchmark_volatility: Optional[int]
    algorithm_period_return: Optional[int]

    @classmethod
    def from_zipline(cls, period):
        return ResultPeriod(
            timestamp=period["period_open"].to_pydatetime().replace(tzinfo=timezone.utc),
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
    periods: List[ResultPeriod]
