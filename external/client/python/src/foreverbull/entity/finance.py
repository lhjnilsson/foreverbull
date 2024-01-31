from datetime import datetime
from enum import IntEnum
from typing import Optional

from .base import Base


class OHLC(Base):
    symbol: str
    open: float
    high: float
    low: float
    close: float
    volume: int
    time: datetime


class Asset(Base):
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


class Order(Base):
    id: Optional[str] = None
    symbol: Optional[str] = None
    amount: Optional[int] = None
    filled: Optional[int] = None
    commission: Optional[float] = None
    limit_price: Optional[int] = None
    stop_price: Optional[int] = None
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


class Position(Base):
    symbol: str
    amount: int
    cost_basis: float


class Portfolio(Base):
    cash: float
    value: float
    positions: list[Position]

    def __contains__(self, asset: Asset) -> bool:
        symbols = [position.symbol for position in self.positions]
        return asset.symbol in symbols

    def __getitem__(self, asset: Asset) -> Position:
        return next(
            (position for position in self.positions if position.symbol == asset.symbol),
            None,
        )
