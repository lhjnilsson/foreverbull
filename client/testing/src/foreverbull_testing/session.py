"""Synthetic OHLC on the fb_database engine.

Tests insert bars. This module does not download prices.
"""

from collections.abc import Mapping
from collections.abc import Sequence
from datetime import UTC
from datetime import datetime
from typing import Any

from sqlalchemy import Connection
from sqlalchemy import text

from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio
from foreverbull.models.service import Algorithm
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2


_OHLC_COLUMNS = ("symbol", "time", "open", "high", "low", "close", "volume")

_DELETE_OHLC = text("DELETE FROM ohlc WHERE symbol = :symbol AND time = :time")

_INSERT_OHLC = text(
    """
    INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
    VALUES (:symbol, :time, :open, :high, :low, :close, :volume)
    """
)


class Session:
    def __init__(self, connection: Connection):
        self._conn = connection
        self._symbols: list[str] = []

    def bars(self, rows: Sequence[Mapping[str, Any]]) -> None:
        """Insert ohlc rows (symbol, time, open, high, low, close, volume)."""
        payload = []
        for row in rows:
            missing = [column for column in _OHLC_COLUMNS if column not in row]
            if missing:
                raise ValueError(f"ohlc row missing {', '.join(missing)}")
            symbol = str(row["symbol"])
            when = _stored_time(row["time"])
            if symbol not in self._symbols:
                self._symbols.append(symbol)
            values = {
                "symbol": symbol,
                "time": when,
                "open": row["open"],
                "high": row["high"],
                "low": row["low"],
                "close": row["close"],
                "volume": int(row["volume"]),
            }
            self._conn.execute(_DELETE_OHLC, {"symbol": symbol, "time": when})
            payload.append(values)
        if payload:
            self._conn.execute(_INSERT_OHLC, payload)

    def assets(self, symbols: Sequence[str], as_of: datetime) -> Assets:
        return Assets(as_of, self._conn, list(symbols))

    def portfolio(
        self,
        as_of: datetime,
        portfolio_value: float,
        positions: Sequence[Any] | None = None,
    ) -> Portfolio:
        return Portfolio(self._portfolio_message(as_of, portfolio_value, positions), self._conn)

    def run(
        self,
        algorithm: Algorithm,
        as_of: datetime,
        portfolio_value: float,
        positions: Sequence[Any] | None = None,
    ):
        message = self._portfolio_message(as_of, portfolio_value, positions)
        functions = algorithm.get_definition().functions
        if not functions:
            raise ValueError("algorithm has no functions")
        orders = []
        symbols = list(self._symbols)
        for function in functions:
            orders.extend(algorithm.process(function.name, self._conn, message, symbols))
        return orders

    def _portfolio_message(
        self,
        as_of: datetime,
        portfolio_value: float,
        positions: Sequence[Any] | None,
    ) -> marketdata_pb2.Portfolio:
        return marketdata_pb2.Portfolio(
            timestamp=pb_utils.to_proto_timestamp(_as_utc(as_of)),
            positions=_proto_positions(positions),
            portfolio_value=portfolio_value,
        )


def _as_utc(as_of: datetime) -> datetime:
    if as_of.tzinfo is None:
        return as_of.replace(tzinfo=UTC)
    return as_of.astimezone(UTC)


def _stored_time(value: datetime) -> datetime:
    # ohlc.time is timestamp without time zone. Keep the UTC wall clock.
    if not isinstance(value, datetime):
        raise TypeError(f"ohlc time must be a datetime, got {type(value).__name__}")
    if value.tzinfo is None:
        return value
    return value.astimezone(UTC).replace(tzinfo=None)


def _position_parts(position: Any) -> tuple[str, int]:
    if isinstance(position, tuple):
        symbol, amount = position
        return str(symbol), int(amount)
    return str(position.symbol), int(position.amount)


def _proto_positions(positions: Sequence[Any] | None) -> list[marketdata_pb2.Position]:
    if not positions:
        return []
    proto: list[marketdata_pb2.Position] = []
    for position in positions:
        if isinstance(position, marketdata_pb2.Position):
            proto.append(position)
            continue
        symbol, amount = _position_parts(position)
        proto.append(marketdata_pb2.Position(symbol=symbol, amount=amount))
    return proto
