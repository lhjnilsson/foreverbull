import math
import os

from datetime import UTC
from datetime import date
from datetime import datetime
from datetime import timedelta

import pandas

from pandas import DataFrame
from pandas import Index
from protobuf import merge_from_binary
from protobuf.wkt import Struct

from foreverbull.broker.utils import namespace_socket
from foreverbull.exceptions import ConfigurationError
from foreverbull.exceptions import RuntimeError
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb as worker_service_pb


# Price source when a backtest names none and OHLC_PROVIDER is unset or blank.
# The CLI and Zipline both read this. Go keeps DefaultProvider in
# pkg/finance/supplier/marketdata.go, and both must name the same provider.
DEFAULT_PROVIDER = "yahoo"

# Missing-price notices. The Zipline ingest fills a session a symbol has no
# bar for with the previous close and writes one line per symbol; the CLI
# shows up to MISSING_PRICES_LINES_SHOWN of them and folds more into one line.
MISSING_PRICES_LINES_SHOWN = 2
# A backtest record (`fbull backtest create` and `get`) names at most this
# many symbols, then "and N more". --verbose lists them all.
BACKTEST_SYMBOLS_SHOWN = 5
_FILLED_WITH_PREVIOUS_CLOSE = "filled with the previous close. No trades on those days."


def missing_prices_line(symbol: str, days: list[date]) -> str:
    """One symbol's notice, e.g. 4 days (2026-05-02 to 2026-05-07)."""
    if not days:
        raise ValueError(f"missing_prices_line needs at least one day for {symbol}")
    ordered = sorted(days)
    if len(ordered) == 1:
        return f"Missing prices for {symbol} on {ordered[0].isoformat()}, {_FILLED_WITH_PREVIOUS_CLOSE}"
    return (
        f"Missing prices for {symbol} on {len(ordered)} days "
        f"({ordered[0].isoformat()} to {ordered[-1].isoformat()}), {_FILLED_WITH_PREVIOUS_CLOSE}"
    )


def missing_prices_summary(lines: list[str], *, verbose: bool) -> list[str]:
    """The lines to print: each symbol's line, or one line for 3+ symbols.

    --verbose prints every symbol's line under the summary.
    """
    if len(lines) <= MISSING_PRICES_LINES_SHOWN:
        return list(lines)
    summary = f"Missing prices for {len(lines)} symbols, {_FILLED_WITH_PREVIOUS_CLOSE}"
    if verbose:
        return [summary, *lines]
    return [f"{summary} Run with --verbose for the list."]


# The clause after the colon. The SQL stays in the worker log.
_MARKET_DATA_DENIED_DETAIL = (
    "the data service can't read market data. This is a bug in the Foreverbull install, not in your strategy."
)


def market_data_denied_line(name: str | None = None) -> str:
    """The one ✖ line for a market-data permission failure.

    Pass the backtest name when the command already has it. An empty name
    leaves the quotes out.
    """
    shown = "" if name is None else str(name).strip()
    if shown:
        return f"Couldn't run backtest '{shown}': {_MARKET_DATA_DENIED_DETAIL}"
    return f"Couldn't run backtest: {_MARKET_DATA_DENIED_DETAIL}"


# What a bar read returns. The CLI adds the backtest name when it prints this.
WORKER_MARKET_DATA_DENIED = market_data_denied_line()


def present_market_data_failure(message: str, name: str | None = None) -> str:
    """The printed line when message is this permission failure, including a wrapped one."""
    if WORKER_MARKET_DATA_DENIED in message:
        return market_data_denied_line(name)
    return message


# One trading year of daily bars is enough for SMA/RSI/MACD without a full-history dump.
_STOCK_DATA_LOOKBACK = 252

# Calendar days a stock_data read reaches back from as_of. 252 trading days
# span about 365 calendar days; twice the lookback plus a month leaves room
# for holidays and halts, so LIMIT still sees the whole lookback while the
# scan no longer grows with every bar stored before it.
_STOCK_DATA_WINDOW = timedelta(days=_STOCK_DATA_LOOKBACK * 2 + 30)

_STOCK_DATA_COLUMNS = ["symbol", "time", "high", "low", "open", "close", "volume"]


def worker_request_error(exc: BaseException) -> str:
    """The text a worker failure returns.

    A permission-denied database error is the install-bug sentence. The
    database error and the SQL stay out of it.
    """
    if _market_data_permission_denied(exc):
        return WORKER_MARKET_DATA_DENIED
    return repr(exc)


def _market_data_permission_denied(exc: BaseException) -> bool:
    seen: set[int] = set()
    current: BaseException | None = exc
    while current is not None and id(current) not in seen:
        seen.add(id(current))
        orig = getattr(current, "orig", None)
        if orig is not None and (_sqlstate(orig) == "42501" or "permission denied" in str(orig).lower()):
            return True
        current = current.__cause__
    return False


def _sqlstate(orig: object) -> str:
    for name in ("sqlstate", "pgcode"):
        value = getattr(orig, name, None)
        if isinstance(value, str) and value != "":
            return value
    return ""


def ohlc_provider() -> str:
    """Price source for bar reads. Unset or blank OHLC_PROVIDER is DEFAULT_PROVIDER (yahoo)."""
    raw = os.environ.get("OHLC_PROVIDER")
    if raw is None:
        return DEFAULT_PROVIDER
    provider = raw.strip().lower()
    if provider == "":
        return DEFAULT_PROVIDER
    return provider


def _as_utc(as_of: datetime) -> datetime:
    if as_of.tzinfo is None:
        return as_of.replace(tzinfo=UTC)
    return as_of.astimezone(UTC)


def _empty_stock_frame() -> DataFrame:
    return DataFrame({name: [] for name in _STOCK_DATA_COLUMNS})


def _bar_time(bar: marketdata_pb.OHLC) -> datetime:
    return _as_utc(pb_utils.from_proto_timestamp(bar.timestamp))


def _frame_for_symbol(bars: list[marketdata_pb.OHLC], symbol: str, as_of: datetime) -> DataFrame:
    """Last lookback bars for symbol with since < time <= as_of, oldest first.

    The broker already resolved ticker aliases. The symbol on each bar is the
    one the algorithm asked for.
    """
    as_of = _as_utc(as_of)
    since = as_of - _STOCK_DATA_WINDOW
    rows = []
    for bar in bars:
        if bar.symbol != symbol:
            continue
        when = _bar_time(bar)
        if when <= as_of and when > since:
            rows.append(
                {
                    "symbol": symbol,
                    "time": when,
                    "high": float(bar.high),
                    "low": float(bar.low),
                    "open": float(bar.open),
                    "close": float(bar.close),
                    "volume": int(bar.volume),
                }
            )
    rows.sort(key=lambda row: row["time"])
    if len(rows) > _STOCK_DATA_LOOKBACK:
        rows = rows[-_STOCK_DATA_LOOKBACK:]
    if not rows:
        return _empty_stock_frame()
    return DataFrame(rows, columns=Index(_STOCK_DATA_COLUMNS))


def _close_on_day(bars: list[marketdata_pb.OHLC], symbol: str, day: date) -> float | None:
    """Close of the latest bar for symbol on that calendar day, in UTC."""
    chosen: tuple[datetime, float] | None = None
    for bar in bars:
        if bar.symbol != symbol:
            continue
        when = _bar_time(bar)
        if when.date() != day:
            continue
        if chosen is None or when >= chosen[0]:
            chosen = (when, float(bar.close))
    if chosen is None:
        return None
    return chosen[1]


def _exchange_namespace(request: worker_service_pb.NamespaceRequest) -> worker_service_pb.NamespaceResponse:
    with namespace_socket() as s:
        s.send(request.to_binary())
        response = worker_service_pb.NamespaceResponse()
        merge_from_binary(response, s.recv())
        if response.has_field("error"):
            raise RuntimeError(response.error)
        return response


def _stock_data_since(as_of: datetime) -> datetime:
    """Exclusive lower bound of the bars a stock_data read scans."""
    return as_of - _STOCK_DATA_WINDOW


class Asset:
    def __init__(
        self,
        as_of: datetime,
        symbol: str,
        bars: list[marketdata_pb.OHLC] | None = None,
        *,
        _stock_data_loader=None,
    ):
        self._as_of = as_of
        self._symbol = symbol
        self._bars = [] if bars is None else bars
        # Set only for Assets iteration. A direct Asset keeps its own frame.
        self._stock_data_loader = _stock_data_loader

    def get_metric[T: (int, float, bool, str, None)](self, key: str) -> T:
        request = worker_service_pb.NamespaceRequest(
            key=key,
            type=worker_service_pb.NamespaceRequestType.GET,
        )
        raw = _exchange_namespace(request).value
        value = {} if raw is None else pb_utils.protobuf_struct_to_dict(raw)
        if self._symbol not in value:
            return None  # type: ignore
        return value[self._symbol]

    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        request = worker_service_pb.NamespaceRequest(
            key=key,
            type=worker_service_pb.NamespaceRequestType.SET,
        )
        request.value = Struct.from_python({self._symbol: value})
        _exchange_namespace(request)

    @property
    def symbol(self):
        return self._symbol

    @property
    def stock_data(self) -> DataFrame:
        if self._stock_data_loader is not None:
            return self._stock_data_loader(self._symbol)
        return _frame_for_symbol(self._bars, self._symbol, self._as_of)


class Assets:
    def __init__(self, as_of: datetime, symbols: list[str], bars: list[marketdata_pb.OHLC] | None = None):
        self._as_of = as_of
        self._symbols = symbols
        self._bars = [] if bars is None else bars
        self._asset_frames: dict[str, DataFrame] | None = None

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> pandas.Series:
        request = worker_service_pb.NamespaceRequest(
            key=key,
            type=worker_service_pb.NamespaceRequestType.GET,
        )
        raw = _exchange_namespace(request).value
        return pandas.Series({} if raw is None else pb_utils.protobuf_struct_to_dict(raw))

    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
        request = worker_service_pb.NamespaceRequest(
            key=key,
            type=worker_service_pb.NamespaceRequestType.SET,
            value=Struct.from_python(dict(value)),
        )
        _exchange_namespace(request)

    @property
    def symbols(self):
        return self._symbols

    def __iter__(self):
        for symbol in self.symbols:
            yield Asset(self._as_of, symbol, self._bars, _stock_data_loader=self._asset_frame)

    def _asset_frame(self, symbol: str) -> DataFrame:
        if self._asset_frames is None:
            self._asset_frames = self._load_asset_frames()
        return self._asset_frames[symbol]

    def _load_asset_frames(self) -> dict[str, DataFrame]:
        # One pass over the bars the broker sent. time ASC is a total order.
        batched = self.stock_data
        empty = batched.iloc[0:0].reset_index()
        if batched.empty:
            present: set[str] = set()
        else:
            present = set(batched.index.get_level_values("symbol"))
        frames: dict[str, DataFrame] = {}
        for symbol in self._symbols:
            if symbol in present:
                frame = batched.loc[[symbol]].reset_index()
                frames[symbol] = frame.sort_values("time", kind="mergesort").reset_index(drop=True)
            else:
                frames[symbol] = empty.copy()
        return frames

    @property
    def stock_data(self) -> DataFrame:
        frames = [_frame_for_symbol(self._bars, symbol, self._as_of) for symbol in self._symbols]
        if not frames:
            empty = _empty_stock_frame()
            empty.set_index(["symbol", "time"], inplace=True)
            return empty
        df = pandas.concat(frames, ignore_index=True)
        df.set_index(["symbol", "time"], inplace=True)
        df.sort_index(inplace=True)
        return df


class Positions:
    def __init__(self, positions: list[marketdata_pb.Position]):
        self._positions = positions
        self._positions_index = 0

    def __iter__(self):
        return self

    def __next__(self) -> marketdata_pb.Position:
        if self._positions_index >= len(self._positions):
            raise StopIteration
        value = self._positions[self._positions_index]
        self._positions_index += 1
        return value

    def __getitem__(self, symbol: str) -> marketdata_pb.Position | None:
        for p in self._positions:
            if p.symbol == symbol:
                return p
        return None

    def __contains__(self, symbol: str) -> bool:
        return self[symbol] is not None


def deployable_value(portfolio_value: float, cash_reserve: float) -> float:
    return portfolio_value * (1.0 - cash_reserve)


def equal_weight_percent(n_selected: int, cash_reserve: float, max_positions: int) -> float:
    # [0, 1) only. NaN fails a plain comparison, and 1 is not a held reserve.
    if math.isnan(cash_reserve) or math.isinf(cash_reserve) or cash_reserve < 0.0 or cash_reserve >= 1.0:
        raise ConfigurationError("cash reserve outside [0, 1)")
    n = min(max_positions, n_selected)
    if n < 1:
        return 0.0
    return (1.0 - cash_reserve) / n


class Portfolio:
    def __init__(self, pb: marketdata_pb.Portfolio, bars: list[marketdata_pb.OHLC] | None = None):
        self._pb = pb
        self._bars = [] if bars is None else bars
        self.pending_orders: list[marketdata_pb.Order] = []
        self._close_by_symbol_day: dict[tuple[str, str], float] = {}

    @property
    def positions(self) -> Positions:
        return Positions(self._pb.positions)

    def deployable_value(self, cash_reserve: float) -> float:
        return deployable_value(self._pb.portfolio_value, cash_reserve)

    def equal_weight_percent(self, n_selected: int, cash_reserve: float, max_positions: int) -> float:
        return equal_weight_percent(n_selected, cash_reserve, max_positions)

    def _close_on_bar(self, symbol: str):
        timestamp = self._pb.timestamp
        if timestamp is None:
            raise ValueError("portfolio has no timestamp")
        day = _as_utc(pb_utils.from_proto_timestamp(timestamp)).date()
        key = (symbol, day.isoformat())
        if key not in self._close_by_symbol_day:
            latest_close = _close_on_day(self._bars, symbol, day)
            if latest_close is None:
                raise ConfigurationError(f"Couldn't find a close price for {symbol} on this bar.")
            self._close_by_symbol_day[key] = latest_close
        return self._close_by_symbol_day[key]

    def _calculate_order_value_amount(self, symbol: str, value: float) -> int:
        # int() truncates toward zero, so a negative value shrinks the share count.
        if value < 0.0:
            raise ConfigurationError("order value is negative")
        close = self._close_on_bar(symbol)
        # A negative close turns a positive value into a sell.
        if close < 0.0:
            raise ConfigurationError("close is negative")
        return int(value / close)

    def _calculate_order_percent_amount(self, symbol: str, percent: float) -> int:
        value = self._pb.portfolio_value * percent
        return self._calculate_order_value_amount(symbol, value)

    def _calculate_order_target_amount(self, symbol: str, amount: int) -> int:
        for pos in self._pb.positions:
            if pos.symbol == symbol:
                return amount - pos.amount
        return amount

    def _enqueue(self, symbol: str, amount: int) -> marketdata_pb.Order:
        order = marketdata_pb.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order(self, symbol: str, amount: int) -> marketdata_pb.Order:
        return self._enqueue(symbol, amount)

    def order_percent(self, symbol: str, percent: float) -> marketdata_pb.Order:
        return self._enqueue(symbol, self._calculate_order_percent_amount(symbol, percent))

    def order_value(self, symbol: str, value: float) -> marketdata_pb.Order:
        return self._enqueue(symbol, self._calculate_order_value_amount(symbol, value))

    def order_target(self, symbol: str, amount: int) -> marketdata_pb.Order:
        return self._enqueue(symbol, self._calculate_order_target_amount(symbol, amount))

    def order_target_percent(self, symbol: str, percent: float) -> marketdata_pb.Order:
        amount = self._calculate_order_percent_amount(symbol, percent)
        return self._enqueue(symbol, self._calculate_order_target_amount(symbol, amount))

    def order_target_value(self, symbol: str, value: float) -> marketdata_pb.Order:
        amount = self._calculate_order_value_amount(symbol, value)
        return self._enqueue(symbol, self._calculate_order_target_amount(symbol, amount))
