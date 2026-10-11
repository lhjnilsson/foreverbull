import os

from datetime import datetime

import pandas

from google.protobuf.internal.containers import RepeatedCompositeFieldContainer
from google.protobuf.struct_pb2 import Struct
from pandas import DataFrame
from pandas import read_sql_query
from sqlalchemy import bindparam
from sqlalchemy import engine
from sqlalchemy import text

from foreverbull.broker.utils import namespace_socket
from foreverbull.exceptions import RuntimeError
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb2 as worker_service_pb2


# One trading year of daily bars is enough for SMA/RSI/MACD without a full-history dump.
_STOCK_DATA_LOOKBACK = 252


def ohlc_provider() -> str:
    """Price source for bar reads. Unset or blank is yahoo, matching the ohlc default."""
    raw = os.environ.get("OHLC_PROVIDER")
    if raw is None:
        return "yahoo"
    provider = raw.strip().lower()
    if provider == "":
        return "yahoo"
    return provider


def _exchange_namespace(request: worker_service_pb2.NamespaceRequest) -> worker_service_pb2.NamespaceResponse:
    with namespace_socket() as s:
        s.send(request.SerializeToString())
        response = worker_service_pb2.NamespaceResponse()
        response.ParseFromString(s.recv())
        if response.HasField("error"):
            raise RuntimeError(response.error)
        return response


class Asset:
    def __init__(self, as_of: datetime, db: engine.Connection, symbol: str, *, _stock_data_loader=None):
        self._as_of = as_of
        self._db = db
        self._symbol = symbol
        # Set only for Assets iteration. A direct Asset keeps the single-symbol query.
        self._stock_data_loader = _stock_data_loader

    def get_metric[T: (int, float, bool, str, None)](self, key: str) -> T:
        request = worker_service_pb2.NamespaceRequest(
            key=key,
            type=worker_service_pb2.NamespaceRequestType.GET,
        )
        value = pb_utils.protobuf_struct_to_dict(_exchange_namespace(request).value)
        if self._symbol not in value:
            return None  # type: ignore
        return value[self._symbol]

    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        request = worker_service_pb2.NamespaceRequest(
            key=key,
            type=worker_service_pb2.NamespaceRequestType.SET,
        )
        request.value.update({self._symbol: value})
        _exchange_namespace(request)

    @property
    def symbol(self):
        return self._symbol

    @property
    def stock_data(self) -> DataFrame:
        if self._stock_data_loader is not None:
            return self._stock_data_loader(self._symbol)
        query = text(
            """
            SELECT symbol, time, high, low, open, close, volume
            FROM (
                SELECT symbol, time, high, low, open, close, volume
                FROM ohlc
                WHERE time <= :as_of AND symbol = :symbol AND provider = :provider
                ORDER BY time DESC, ctid DESC
                LIMIT :limit
            ) recent
            ORDER BY time ASC
            """
        ).bindparams(as_of=self._as_of, symbol=self.symbol, provider=ohlc_provider(), limit=_STOCK_DATA_LOOKBACK)
        return read_sql_query(query, self._db)


class Assets:
    def __init__(self, as_of: datetime, db: engine.Connection, symbols: list[str]):
        self._as_of = as_of
        self._db = db
        self._symbols = symbols
        self._asset_frames: dict[str, DataFrame] | None = None

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> pandas.Series:
        request = worker_service_pb2.NamespaceRequest(
            key=key,
            type=worker_service_pb2.NamespaceRequestType.GET,
        )
        return pandas.Series(pb_utils.protobuf_struct_to_dict(_exchange_namespace(request).value))

    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
        struct = Struct()
        for k, v in value.items():
            struct.update({k: v})
        request = worker_service_pb2.NamespaceRequest(
            key=key,
            type=worker_service_pb2.NamespaceRequestType.SET,
            value=struct,
        )
        _exchange_namespace(request)

    @property
    def symbols(self):
        return self._symbols

    def __iter__(self):
        for symbol in self.symbols:
            yield Asset(self._as_of, self._db, symbol, _stock_data_loader=self._asset_frame)

    def _asset_frame(self, symbol: str) -> DataFrame:
        if self._asset_frames is None:
            self._asset_frames = self._load_asset_frames()
        return self._asset_frames[symbol]

    def _load_asset_frames(self) -> dict[str, DataFrame]:
        # One statement. ohlc is UNIQUE(provider, symbol, time), so this window matches
        # Asset.stock_data's ORDER BY time DESC LIMIT, and time ASC is a total order.
        batched = self.stock_data
        empty = batched.iloc[0:0].reset_index()
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
        query = text(
            """
            SELECT symbol, time, high, low, open, close, volume
            FROM (
                SELECT symbol, time, high, low, open, close, volume,
                       ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY time DESC, ctid DESC) AS rn
                FROM ohlc
                WHERE time <= :as_of AND symbol IN :symbols AND provider = :provider
            ) bounded
            WHERE rn <= :limit
            ORDER BY symbol, time
            """
        ).bindparams(
            bindparam("as_of", value=self._as_of),
            bindparam("symbols", value=list(self.symbols), expanding=True),
            bindparam("provider", value=ohlc_provider()),
            bindparam("limit", value=_STOCK_DATA_LOOKBACK),
        )
        df = read_sql_query(query, self._db)
        df.set_index(["symbol", "time"], inplace=True)
        df.sort_index(inplace=True)
        return df


class Positions:
    def __init__(self, positions: RepeatedCompositeFieldContainer[marketdata_pb2.Position]):
        self._positions = positions
        self._positions_index = 0

    def __iter__(self):
        return self

    def __next__(self) -> marketdata_pb2.Position:
        if self._positions_index >= len(self._positions):
            raise StopIteration
        value = self._positions[self._positions_index]
        self._positions_index += 1
        return value

    def __getitem__(self, symbol: str) -> marketdata_pb2.Position | None:
        for p in self._positions:
            if p.symbol == symbol:
                return p
        return None

    def __contains__(self, symbol: str) -> bool:
        return self[symbol] is not None


def deployable_value(portfolio_value: float, cash_reserve: float) -> float:
    return portfolio_value * (1.0 - cash_reserve)


def equal_weight_percent(n_selected: int, cash_reserve: float, max_positions: int) -> float:
    n = min(max_positions, n_selected)
    if n < 1:
        return 0.0
    return (1.0 - cash_reserve) / n


class Portfolio:
    def __init__(self, pb: marketdata_pb2.Portfolio, db: engine.Connection):
        self._pb = pb
        self._db = db
        self.pending_orders: list[marketdata_pb2.Order] = []
        self._close_by_symbol_day: dict[tuple[str, str, str], float] = {}

    @property
    def positions(self) -> Positions:
        return Positions(self._pb.positions)

    def deployable_value(self, cash_reserve: float) -> float:
        return deployable_value(self._pb.portfolio_value, cash_reserve)

    def equal_weight_percent(self, n_selected: int, cash_reserve: float, max_positions: int) -> float:
        return equal_weight_percent(n_selected, cash_reserve, max_positions)

    def _close_on_bar(self, symbol: str):
        day = self._pb.timestamp.ToDatetime().strftime("%Y-%m-%d")
        provider = ohlc_provider()
        key = (provider, symbol, day)
        if key not in self._close_by_symbol_day:
            q = text("SELECT close FROM ohlc WHERE symbol=:symbol AND provider=:provider AND time::DATE=:dt")
            q = q.bindparams(symbol=symbol, provider=provider, dt=day)
            latest_close = self._db.execute(q).scalar()
            assert latest_close is not None, f"unable to find latest close price for {symbol}"
            self._close_by_symbol_day[key] = float(latest_close)
        return self._close_by_symbol_day[key]

    def _calculate_order_value_amount(self, symbol: str, value: float) -> int:
        return int(value / self._close_on_bar(symbol))

    def _calculate_order_percent_amount(self, symbol: str, percent: float) -> int:
        value = self._pb.portfolio_value * percent
        return self._calculate_order_value_amount(symbol, value)

    def _calculate_order_target_amount(self, symbol: str, amount: int) -> int:
        for pos in self._pb.positions:
            if pos.symbol == symbol:
                return amount - pos.amount
        return amount

    def _enqueue(self, symbol: str, amount: int) -> marketdata_pb2.Order:
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order(self, symbol: str, amount: int) -> marketdata_pb2.Order:
        return self._enqueue(symbol, amount)

    def order_percent(self, symbol: str, percent: float) -> marketdata_pb2.Order:
        return self._enqueue(symbol, self._calculate_order_percent_amount(symbol, percent))

    def order_value(self, symbol: str, value: float) -> marketdata_pb2.Order:
        return self._enqueue(symbol, self._calculate_order_value_amount(symbol, value))

    def order_target(self, symbol: str, amount: int) -> marketdata_pb2.Order:
        return self._enqueue(symbol, self._calculate_order_target_amount(symbol, amount))

    def order_target_percent(self, symbol: str, percent: float) -> marketdata_pb2.Order:
        amount = self._calculate_order_percent_amount(symbol, percent)
        return self._enqueue(symbol, self._calculate_order_target_amount(symbol, amount))

    def order_target_value(self, symbol: str, value: float) -> marketdata_pb2.Order:
        amount = self._calculate_order_value_amount(symbol, value)
        return self._enqueue(symbol, self._calculate_order_target_amount(symbol, amount))
