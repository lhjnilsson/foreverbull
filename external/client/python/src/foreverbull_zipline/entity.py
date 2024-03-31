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


class Period(Base):
    timestamp: datetime
    cash_flow: float
    starting_cash: float
    portfolio_value: float
    pnl: float
    returns: float
    cash: float
    positions_value: float
    positions_exposure: float

    positions: List[Position]
    new_orders: List[Order]

    @classmethod
    def from_zipline(cls, trading_algorithm, new_orders):
        return cls(
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


class Result(Base):
    class Period(Base):
        timestamp: datetime
        pnl: float
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

        # Can be None on initial periods
        algo_volatility: Optional[float]
        sharpe: Optional[float]
        sortino: Optional[float]
        # Only in benchmark
        benchmark_period_return: Optional[float]
        benchmark_volatility: Optional[float]
        alpha: Optional[float]
        beta: Optional[float]

        @classmethod
        def from_zipline(cls, period):
            return cls(
                timestamp=period["period_open"].to_pydatetime().replace(tzinfo=timezone.utc),
                pnl=period["pnl"],
                returns=period["returns"],
                portfolio_value=period["portfolio_value"],
                longs_count=period["longs_count"],
                shorts_count=period["shorts_count"],
                long_value=period["long_value"],
                short_value=period["short_value"],
                starting_exposure=period["starting_exposure"],
                ending_exposure=period["ending_exposure"],
                long_exposure=period["long_exposure"],
                short_exposure=period["short_exposure"],
                capital_used=period["capital_used"],
                gross_leverage=period["gross_leverage"],
                net_leverage=period["net_leverage"],
                starting_value=period["starting_value"],
                ending_value=period["ending_value"],
                starting_cash=period["starting_cash"],
                ending_cash=period["ending_cash"],
                max_drawdown=period["max_drawdown"],
                max_leverage=period["max_leverage"],
                excess_return=period["excess_return"],
                treasury_period_return=period["treasury_period_return"],
                algorithm_period_return=period["algorithm_period_return"],
                algo_volatility=None if pd.isnull(period["algo_volatility"]) else period["algo_volatility"],
                sharpe=None if pd.isnull(period["sharpe"]) else period["sharpe"],
                sortino=None if pd.isnull(period["sortino"]) else period["sortino"],
                benchmark_period_return=(
                    None if pd.isnull(period["benchmark_period_return"]) else period["benchmark_period_return"]
                ),
                benchmark_volatility=(
                    None if pd.isnull(period["benchmark_volatility"]) else period["benchmark_volatility"]
                ),
                alpha=None if period["alpha"] is None or pd.isnull(period["alpha"]) else period["alpha"],
                beta=None if period["beta"] is None or pd.isnull(period["beta"]) else period["beta"],
            )

    periods: List[Period]

    @classmethod
    def from_zipline(cls, result: pd.DataFrame):
        periods = []
        for row in result.index:
            periods.append(cls.Period.from_zipline(result.loc[row]))
        return cls(periods=periods)
