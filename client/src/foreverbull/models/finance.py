from datetime import datetime

import pandas

from google.protobuf.internal.containers import RepeatedCompositeFieldContainer
from google.protobuf.struct_pb2 import Struct
from pandas import DataFrame
from pandas import read_sql_query
from sqlalchemy import engine
from sqlalchemy import text

from foreverbull.broker.utils import namespace_socket
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb2 as worker_service_pb2


class Asset:
    def __init__(self, as_of: datetime, db: engine.Connection, symbol: str):
        self._as_of = as_of
        self._db = db
        self._symbol = symbol

    def get_metric[T: (int, float, bool, str, None)](self, key: str) -> T:
        with namespace_socket() as s:
            request = worker_service_pb2.NamespaceRequest(
                key=key,
                type=worker_service_pb2.NamespaceRequestType.GET,
            )
            s.send(request.SerializeToString())
            response = worker_service_pb2.NamespaceResponse()
            response.ParseFromString(s.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            value = pb_utils.protobuf_struct_to_dict(response.value)
            if self._symbol not in value:
                return None  # type: ignore
            return value[self._symbol]

    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        with namespace_socket() as s:
            request = worker_service_pb2.NamespaceRequest(
                key=key,
                type=worker_service_pb2.NamespaceRequestType.SET,
            )
            request.value.update({self._symbol: value})
            s.send(request.SerializeToString())
            response = worker_service_pb2.NamespaceResponse()
            response.ParseFromString(s.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            return None

    @property
    def symbol(self):
        return self._symbol

    @property
    def stock_data(self) -> DataFrame:
        return read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time <= '{self._as_of}' AND symbol='{self.symbol}'""",
            self._db,
        )


class Assets:
    def __init__(self, as_of: datetime, db: engine.Connection, symbols: list[str]):
        self._as_of = as_of
        self._db = db
        self._symbols = symbols

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> pandas.Series:
        with namespace_socket() as s:
            request = worker_service_pb2.NamespaceRequest(
                key=key,
                type=worker_service_pb2.NamespaceRequestType.GET,
            )
            s.send(request.SerializeToString())
            response = worker_service_pb2.NamespaceResponse()
            response.ParseFromString(s.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            return pandas.Series(pb_utils.protobuf_struct_to_dict(response.value))

    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
        with namespace_socket() as s:
            struct = Struct()
            for k, v in value.items():
                struct.update({k: v})
            request = worker_service_pb2.NamespaceRequest(
                key=key,
                type=worker_service_pb2.NamespaceRequestType.SET,
                value=struct,
            )
            s.send(request.SerializeToString())
            response = worker_service_pb2.NamespaceResponse()
            response.ParseFromString(s.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            return None

    @property
    def symbols(self):
        return self._symbols

    def __iter__(self):
        for symbol in self.symbols:
            yield Asset(self._as_of, self._db, symbol)

    @property
    def stock_data(self) -> DataFrame:
        df = read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time <= '{self._as_of}' AND symbol IN {tuple(self.symbols) if len(self.symbols) > 1 else f"('{self.symbols[0]}')"}""",
            self._db,
        )
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


class Portfolio:
    def __init__(self, pb: marketdata_pb2.Portfolio, db: engine.Connection):
        self._pb = pb
        self._db = db
        self.pending_orders: list[marketdata_pb2.Order] = []

    @property
    def positions(self) -> Positions:
        return Positions(self._pb.positions)

    def _calculate_order_value_amount(self, symbol: str, value: float) -> int:
        q = text("SELECT close FROM ohlc WHERE symbol=:symbol and time::DATE=:dt")
        q = q.bindparams(symbol=symbol, dt=self._pb.timestamp.ToDatetime().strftime("%Y-%m-%d"))
        latest_close = self._db.execute(q).scalar()
        assert latest_close is not None, f"unable to find latest close price for {symbol}"
        return int(value / float(latest_close))

    def _calculate_order_percent_amount(self, symbol: str, percent: float) -> int:
        value = self._pb.portfolio_value * percent
        return self._calculate_order_value_amount(symbol, value)

    def _calculate_order_target_amount(self, symbol: str, amount: int) -> int:
        for pos in self._pb.positions:
            if pos.symbol == symbol:
                return amount - pos.amount
        return amount

    def order(self, symbol: str, amount: int) -> marketdata_pb2.Order:
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_percent(self, symbol: str, percent: float) -> marketdata_pb2.Order:
        amount = self._calculate_order_percent_amount(symbol, percent)
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_value(self, symbol: str, value: float) -> marketdata_pb2.Order:
        amount = self._calculate_order_value_amount(symbol, value)
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_target(self, symbol: str, amount: int) -> marketdata_pb2.Order:
        amount = self._calculate_order_target_amount(symbol, amount)
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_target_percent(self, symbol: str, percent: float) -> marketdata_pb2.Order:
        amount = self._calculate_order_percent_amount(symbol, percent)
        amount = self._calculate_order_target_amount(symbol, amount)
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_target_value(self, symbol: str, value: float) -> marketdata_pb2.Order:
        amount = self._calculate_order_value_amount(symbol, value)
        amount = self._calculate_order_target_amount(symbol, amount)
        order = marketdata_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order
