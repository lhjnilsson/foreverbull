import builtins
import importlib.util
import logging
import os
import re

from contextlib import contextmanager
from datetime import datetime
from functools import partial
from inspect import getabsfile
from inspect import signature
from typing import Callable
from typing import Iterator

import pandas
import pynng

from google.protobuf.internal.containers import RepeatedCompositeFieldContainer
from google.protobuf.struct_pb2 import Struct
from pandas import DataFrame
from pandas import read_sql_query
from sqlalchemy import Connection
from sqlalchemy import create_engine
from sqlalchemy import engine
from sqlalchemy import text

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance import finance_pb2  # noqa
from foreverbull.pb.foreverbull.service import worker_pb2
from foreverbull.pb.foreverbull.service import worker_service_pb2


# Hacky way to get the database URL, TODO: find a better way
def get_engine(url: str):
    log = logging.getLogger().getChild(__name__)

    if url.startswith("postgres://"):
        url = url.replace("postgres://", "postgresql://", 1)

    try:
        engine = create_engine(url)
        engine.connect()
        return engine
    except Exception:
        log.warning(f"Could not connect to {url}")

    database_host = re.search(r"@([^/]+):", url)
    if database_host is None:
        raise Exception("Could not find database host in URL")
    database_host = database_host.group(1)
    database_port = re.search(r":(\d+)/", url)
    if database_port is None:
        raise Exception("Could not find database port in URL")
    database_port = database_port.group(1)

    new_url = ""
    for hostname in ["localhost", "postgres", "127.0.0.1"]:
        for port in [database_port, "5432"]:
            try:
                new_url = url.replace(f"@{database_host}:", f"@{hostname}:", 1)
                new_url = new_url.replace(f":{database_port}", ":5432", 1)
                engine = create_engine(new_url)
                engine.connect()
                log.info(f"Connected to {new_url}")
                return engine
            except Exception:
                log.warning(f"Could not connect to {new_url}")
    raise Exception("Could not connect to database")


@contextmanager
def namespace_socket() -> Iterator[pynng.Socket]:
    hostname = os.environ.get("BROKER_HOSTNAME", "127.0.0.1")
    port = os.environ.get("NAMESPACE_PORT", None)
    if port is None:
        raise Exception("Namespace port not set")
    socket = pynng.Req0(dial=f"tcp://{hostname}:{port}", block_on_dial=True)
    socket.recv_timeout = 500
    socket.send_timeout = 500
    yield socket
    socket.close()


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
            FROM ohlc WHERE time <= '{self._as_of}' AND symbol IN {tuple(self.symbols) if len(self.symbols) > 1 else f"('{self.symbols[0]}')" }""",
            self._db,
        )
        df.set_index(["symbol", "time"], inplace=True)
        df.sort_index(inplace=True)
        return df


class Positions:
    def __init__(self, positions: RepeatedCompositeFieldContainer[finance_pb2.Position]):
        self._positions = positions
        self._positions_index = 0

    def __iter__(self):
        return self

    def __next__(self) -> finance_pb2.Position:
        if self._positions_index >= len(self._positions):
            raise StopIteration
        value = self._positions[self._positions_index]
        self._positions_index += 1
        return value

    def __getitem__(self, symbol: str) -> finance_pb2.Position | None:
        for p in self._positions:
            if p.symbol == symbol:
                return p
        return None

    def __contains__(self, symbol: str) -> bool:
        return self[symbol] is not None


class Portfolio:
    def __init__(self, pb: finance_pb2.Portfolio, db: engine.Connection):
        self._pb = pb
        self._db = db
        self.pending_orders: list[finance_pb2.Order] = []

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

    def order(self, symbol: str, amount: int) -> finance_pb2.Order:
        order = finance_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_percent(self, symbol: str, percent: float) -> finance_pb2.Order:
        amount = self._calculate_order_percent_amount(symbol, percent)
        order = finance_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_value(self, symbol: str, value: float) -> finance_pb2.Order:
        amount = self._calculate_order_value_amount(symbol, value)
        order = finance_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_target(self, symbol: str, amount: int) -> finance_pb2.Order:
        amount = self._calculate_order_target_amount(symbol, amount)
        order = finance_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_target_percent(self, symbol: str, percent: float) -> finance_pb2.Order:
        amount = self._calculate_order_percent_amount(symbol, percent)
        amount = self._calculate_order_target_amount(symbol, amount)
        order = finance_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order

    def order_target_value(self, symbol: str, value: float) -> finance_pb2.Order:
        amount = self._calculate_order_value_amount(symbol, value)
        amount = self._calculate_order_target_amount(symbol, amount)
        order = finance_pb2.Order(symbol=symbol, amount=amount)
        self.pending_orders.append(order)
        return order


class Function:
    def __init__(self, callable: Callable, run_first: bool = False, run_last: bool = False):
        self.callable = callable
        self.run_first = run_first
        self.run_last = run_last


class Algorithm:
    _algo: "Algorithm | None"
    _file_path: str
    _functions: dict
    _namespaces: list[str]

    def __init__(self, functions: list[Function], namespaces: list[str] = []):
        Algorithm._algo = None
        Algorithm._file_path = getabsfile(functions[0].callable)
        Algorithm._functions = {}
        Algorithm._namespaces = namespaces

        for f in functions:
            parameters = []
            asset_key = None
            portfolio_key = None
            parallel_execution: bool | None = None

            for key, value in signature(f.callable).parameters.items():
                if value.annotation == Portfolio:
                    portfolio_key = key
                    continue
                if issubclass(value.annotation, Assets):
                    parallel_execution = False
                    asset_key = key
                elif issubclass(value.annotation, Asset):
                    parallel_execution = True
                    asset_key = key
                else:
                    default = None if value.default == value.empty else str(value.default)
                    parameter = worker_pb2.Algorithm.FunctionParameter(
                        key=key,
                        defaultValue=default,
                        valueType=self.type_to_str(value.annotation),
                    )
                    parameters.append(parameter)
            if parallel_execution is None:
                raise TypeError("Function {} must have a parameter of type Asset or Assets".format(f.callable.__name__))
            function = {
                "callable": f.callable,
                "asset_key": asset_key,
                "portfolio_key": portfolio_key,
                "definition": worker_pb2.Algorithm.Function(
                    name=f.callable.__name__,  # type: ignore
                    parameters=parameters,
                    parallelExecution=parallel_execution,
                    runFirst=f.run_first,
                    runLast=f.run_last,
                ),
            }

            Algorithm._functions[f.callable.__name__] = function
        Algorithm._algo = self

    def get_definition(self) -> worker_pb2.Algorithm:
        functions: list[worker_pb2.Algorithm.Function] = []
        for name, function in Algorithm._functions.items():
            functions.append(function["definition"])
        return worker_pb2.Algorithm(
            file_path=Algorithm._file_path,
            functions=functions,
            namespaces=Algorithm._namespaces,
        )

    @staticmethod
    def type_to_str[T: (int, float, bool, str)](t: T) -> str:
        match t:
            case builtins.int:
                return "int"
            case builtins.float:
                return "float"
            case builtins.bool:
                return "bool"
            case builtins.str:
                return "string"
            case _:
                raise TypeError("Unsupported type: ", type(t))

    @classmethod
    def from_file_path(cls, file_path: str) -> "Algorithm":
        spec = importlib.util.spec_from_file_location(
            "",
            file_path,
        )
        if spec is None:
            raise Exception("No spec found in {}".format(file_path))
        if spec.loader is None:
            raise Exception("No loader found in {}".format(file_path))
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)
        if Algorithm._algo is None:
            raise Exception("No algo found in {}".format(file_path))
        return Algorithm._algo

    def configure(self, function_name: str, param_key: str, param_value: str) -> None:
        def _eval_param(type: str, val: str):
            if type == "int":
                return int(val)
            elif type == "float":
                return float(val)
            elif type == "bool":
                return bool(val)
            elif type == "str":
                return str(val)
            else:
                raise TypeError(f"Unknown parameter type: {type}")

        param_type: str = ""
        for f in Algorithm._functions.values():
            if f["definition"].name == function_name:
                function_name = f["definition"].name
                for p in f["definition"].parameters:
                    if p.key == param_key:
                        if not p.valueType:
                            raise TypeError("unable to determine parameter type")
                        param_type = p.valueType
                        break
                else:
                    raise TypeError(f"Unknown parameter: {param_key}")
                break

        value = _eval_param(param_type, param_value)
        function = Algorithm._functions[function_name]
        Algorithm._functions[function_name]["callable"] = partial(
            function["callable"],
            **{param_key: value},
        )

    def process(
        self,
        function_name: str,
        db: Connection,
        portfolio: finance_pb2.Portfolio,
        symbols: list[str],
    ) -> list[finance_pb2.Order]:
        p = Portfolio(portfolio, db)
        if Algorithm._functions[function_name]["definition"].parallelExecution:
            for symbol in symbols:
                asset = Asset(pb_utils.from_proto_timestamp(portfolio.timestamp), db, symbol)
                Algorithm._functions[function_name]["callable"](
                    asset=asset,
                    portfolio=p,
                )
        else:
            assets = Assets(pb_utils.from_proto_timestamp(portfolio.timestamp), db, symbols)
            Algorithm._functions[function_name]["callable"](assets=assets, portfolio=p)
        return p.pending_orders
