"""Synthetic OHLC on the fb_database engine.

Tests insert bars. This module does not download prices.
"""

from collections.abc import Mapping
from collections.abc import Sequence
from datetime import UTC
from datetime import datetime
from datetime import timedelta
from typing import Any
from typing import NamedTuple

from sqlalchemy import Connection
from sqlalchemy import text

from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio
from foreverbull.models.finance import ohlc_provider
from foreverbull.models.service import Algorithm
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb


_OHLC_COLUMNS = ("symbol", "time", "open", "high", "low", "close", "volume")

# Monday, so a series can end on a weekday the production time <= as_of query includes.
_SERIES_END = datetime(2024, 5, 6, tzinfo=UTC)

_DELETE_SYMBOL = text("DELETE FROM ohlc WHERE symbol = :symbol AND provider = :provider")

_DELETE_OHLC = text("DELETE FROM ohlc WHERE symbol = :symbol AND provider = :provider AND time = :time")

_INSERT_OHLC = text(
    """
    INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
    VALUES (:provider, :symbol, :time, :open, :high, :low, :close, :volume)
    """
)

_CLOSE_ON_DAY = text("SELECT close FROM ohlc WHERE symbol=:symbol AND provider=:provider AND time::DATE=:dt")


class ReplayDay(NamedTuple):
    """Post-fill snapshot for one as_of. An order trace for tests, not a broker or Zipline result."""

    as_of: datetime
    orders: tuple[Any, ...]
    positions: dict[str, int]
    portfolio_value: float
    cash: float


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
            provider = ohlc_provider()
            values = {
                "provider": provider,
                "symbol": symbol,
                "time": when,
                "open": row["open"],
                "high": row["high"],
                "low": row["low"],
                "close": row["close"],
                "volume": int(row["volume"]),
            }
            self._conn.execute(_DELETE_OHLC, {"symbol": symbol, "provider": provider, "time": when})
            payload.append(values)
        if payload:
            self._conn.execute(_INSERT_OHLC, payload)

    def constant(self, symbol: str, days: int, price: float) -> list[datetime]:
        """Replace this symbol with `days` flat weekday closes. Returns bar times, oldest first."""
        close = _positive_price("price", price)
        return self._insert_closes(symbol, [close] * _positive_days(days))

    def trend(self, symbol: str, days: int, start_price: float, end_price: float) -> list[datetime]:
        """Replace this symbol with monotone weekday closes from start_price to end_price."""
        count = _positive_days(days)
        start = _positive_price("start_price", start_price)
        end = _positive_price("end_price", end_price)
        if count == 1 and start != end:
            raise ValueError("trend of one day needs start_price == end_price")
        return self._insert_closes(symbol, _monotone_closes(start, end, count))

    def cross_up(self, symbol: str, short: int, long: int) -> list[datetime]:
        """Replace this symbol with weekday bars that cross up on the last close.

        At least `long` positive closes. The last bar is the first where the
        short rolling mean exceeds the long rolling mean.
        """
        short_window = _positive_window("short", short)
        long_window = _positive_window("long", long)
        if short_window >= long_window:
            raise ValueError("short window must be less than long window")
        return self._insert_closes(symbol, _cross_up_closes(short_window, long_window))

    def _insert_closes(self, symbol: str, closes: list[float]) -> list[datetime]:
        times = _weekday_timestamps(len(closes))
        self._conn.execute(_DELETE_SYMBOL, {"symbol": symbol, "provider": ohlc_provider()})
        self.bars(
            [
                {
                    "symbol": symbol,
                    "time": when,
                    "open": close,
                    "high": close,
                    "low": close,
                    "close": close,
                    "volume": 1,
                }
                for when, close in zip(times, closes, strict=True)
            ]
        )
        return times

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

    def replay(
        self,
        algorithm: Algorithm,
        as_of_dates: Sequence[datetime],
        portfolio_value: float,
    ) -> list[ReplayDay]:
        """Walk inserted dates and trace orders. Not a broker or Zipline result.

        For each day, call Algorithm.process with that day's as_of and the carried
        positions. Pending order.amount is a delta, not a target: add it to the
        position and fill at that day's close. No slippage and no commission.
        Cash starts at portfolio_value when there are no positions. The next
        portfolio_value is cash plus the sum of shares times that day's close.
        """
        cash = float(portfolio_value)
        value = cash
        held: dict[str, int] = {}
        trace: list[ReplayDay] = []
        for as_of in as_of_dates:
            orders = tuple(self.run(algorithm, as_of, value, list(held.items())))
            for order in orders:
                amount = int(order.amount)
                if amount == 0:
                    continue
                symbol = str(order.symbol)
                close = self._close_on(symbol, as_of)
                held[symbol] = held.get(symbol, 0) + amount
                cash -= amount * close
                if held[symbol] == 0:
                    del held[symbol]
            market_value = 0.0
            for symbol, shares in held.items():
                market_value += shares * self._close_on(symbol, as_of)
            value = cash + market_value
            trace.append(
                ReplayDay(
                    as_of=as_of,
                    orders=orders,
                    positions=dict(held),
                    portfolio_value=value,
                    cash=cash,
                )
            )
        return trace

    def _close_on(self, symbol: str, as_of: datetime) -> float:
        day = _as_utc(as_of).strftime("%Y-%m-%d")
        close = self._conn.execute(_CLOSE_ON_DAY, {"symbol": symbol, "provider": ohlc_provider(), "dt": day}).scalar()
        if close is None:
            raise ValueError(f"no close for {symbol} on {day}")
        return float(close)

    def _portfolio_message(
        self,
        as_of: datetime,
        portfolio_value: float,
        positions: Sequence[Any] | None,
    ) -> marketdata_pb.Portfolio:
        return marketdata_pb.Portfolio(
            timestamp=pb_utils.to_proto_timestamp(_as_utc(as_of)),
            positions=_proto_positions(positions),
            portfolio_value=portfolio_value,
        )


def _positive_days(days: int) -> int:
    if isinstance(days, bool) or not isinstance(days, int) or days < 1:
        raise ValueError("days must be a positive integer")
    return days


def _positive_window(label: str, window: int) -> int:
    if isinstance(window, bool) or not isinstance(window, int) or window < 1:
        raise ValueError(f"{label} window must be a positive integer")
    return window


def _positive_price(label: str, price: float) -> float:
    value = float(price)
    if not value > 0:
        raise ValueError(f"{label} must be positive")
    return value


def _weekday_timestamps(count: int) -> list[datetime]:
    found: list[datetime] = []
    day = _SERIES_END
    while len(found) < count:
        if day.weekday() < 5:
            found.append(day)
        day -= timedelta(days=1)
    found.reverse()
    return found


def _monotone_closes(start: float, end: float, count: int) -> list[float]:
    if count == 1:
        return [end]
    closes = [start + (end - start) * i / (count - 1) for i in range(count)]
    closes[-1] = end
    return closes


def _window_mean(closes: list[float], index: int, window: int) -> float | None:
    start = index - window + 1
    if start < 0:
        return None
    return sum(closes[start : index + 1]) / window


def _cross_up_closes(short: int, long: int) -> list[float]:
    # Flat history, then one higher close. The shorter mean reacts first.
    closes = [10.0] * long + [20.0]
    last = len(closes) - 1
    short_prev = _window_mean(closes, last - 1, short)
    long_prev = _window_mean(closes, last - 1, long)
    short_last = _window_mean(closes, last, short)
    long_last = _window_mean(closes, last, long)
    if short_prev is None or long_prev is None or short_last is None or long_last is None:
        raise RuntimeError("cross_up windows are not defined on the last two bars")
    if short_prev > long_prev or short_last <= long_last:
        raise RuntimeError("cross_up closes did not cross on the last bar")
    for index in range(last):
        short_mean = _window_mean(closes, index, short)
        long_mean = _window_mean(closes, index, long)
        if short_mean is not None and long_mean is not None and short_mean > long_mean:
            raise RuntimeError("cross_up crossed before the last bar")
    return closes


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


def _proto_positions(positions: Sequence[Any] | None) -> list[marketdata_pb.Position]:
    if not positions:
        return []
    proto: list[marketdata_pb.Position] = []
    for position in positions:
        if isinstance(position, marketdata_pb.Position):
            proto.append(position)
            continue
        symbol, amount = _position_parts(position)
        proto.append(marketdata_pb.Position(symbol=symbol, amount=amount))
    return proto
