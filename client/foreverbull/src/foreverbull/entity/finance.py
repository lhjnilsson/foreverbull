from datetime import datetime
from enum import IntEnum
from typing import Optional

import pydantic


class OHLC(pydantic.BaseModel):
    symbol: str
    open: float
    high: float
    low: float
    close: float
    volume: int
    time: datetime


class Asset(pydantic.BaseModel):
    symbol: str
    name: Optional[str] = None
    title: Optional[str] = None
    asset_type: Optional[str] = None


class OrderStatus(IntEnum):
    OPEN = 0
    FILLED = 1
    CANCELLED = 2
    REJECTED = 3
    HELD = 4


class Order(pydantic.BaseModel):
    id: Optional[str] = None
    symbol: Optional[str] = None
    amount: Optional[int] = None
    filled: Optional[int] = None
    commission: Optional[float] = None
    limit_price: Optional[float] = None
    stop_price: Optional[float] = None
    created_at: Optional[datetime] = None
    status: Optional[OrderStatus] = None

    @classmethod
    def from_zipline(cls, order):
        return cls(
            id=order.id,
            symbol=order.sid.symbol,
            amount=order.amount,
            filled=order.filled,
            commission=order.commission,
            limit_price=order.limit,
            stop_price=order.stop,
            created_at=order.created,
            status=order.status,
        )


class Position(pydantic.BaseModel):
    symbol: str
    amount: int
    cost_basis: float


class Portfolio(pydantic.BaseModel):
    cash: float
    portfolio_value: float
    positions: list[Position]
