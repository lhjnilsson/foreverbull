import logging
import os
import re
from contextlib import contextmanager
from datetime import datetime
from typing import Iterator

import pynng
from foreverbull import entity, interfaces
from foreverbull.pb import pb_utils
from foreverbull.pb.finance import finance_pb2  # noqa
from foreverbull.pb.service import service_pb2
from google.protobuf.struct_pb2 import Struct
from pandas import DataFrame, read_sql_query
from sqlalchemy import create_engine, engine


# Hacky way to get the database URL, TODO: find a better way
def get_engine(url: str):
    log = logging.getLogger(__name__)

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


class Asset(interfaces.Asset):
    def __init__(self, as_of: datetime, db: engine.Connection, symbol: str):
        self._as_of = as_of
        self._db = db
        self._symbol = symbol

    def get_metric[T: (int, float, bool, str, None)](self, key: str) -> T:
        with namespace_socket() as s:
            request = service_pb2.NamespaceRequest(
                key=key,
                type=service_pb2.NamespaceRequestType.GET,
            )
            s.send(request.SerializeToString())
            response = service_pb2.NamespaceResponse()
            response.ParseFromString(s.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            value = pb_utils.protobuf_struct_to_dict(response.value)
            if self._symbol not in value:
                return None
            return value[self._symbol]

    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        with namespace_socket() as s:
            request = service_pb2.NamespaceRequest(
                key=key,
                type=service_pb2.NamespaceRequestType.SET,
            )
            request.value.update({self._symbol: value})
            s.send(request.SerializeToString())
            response = service_pb2.NamespaceResponse()
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


class Assets(interfaces.Assets):
    def __init__(self, as_of: datetime, db: engine.Connection, symbols: list[str]):
        self._as_of = as_of
        self._db = db
        self._symbols = symbols

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> dict[str, T]:
        with namespace_socket() as s:
            request = service_pb2.NamespaceRequest(
                key=key,
                type=service_pb2.NamespaceRequestType.GET,
            )
            s.send(request.SerializeToString())
            response = service_pb2.NamespaceResponse()
            response.ParseFromString(s.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            return pb_utils.protobuf_struct_to_dict(response.value)

    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
        with namespace_socket() as s:
            struct = Struct()
            for k, v in value.items():
                struct.update({k: v})
            request = service_pb2.NamespaceRequest(
                key=key,
                type=service_pb2.NamespaceRequestType.SET,
                value=struct,
            )
            s.send(request.SerializeToString())
            response = service_pb2.NamespaceResponse()
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
        return DataFrame()  # TODO: Implement


class Portfolio(entity.finance.Portfolio):
    def __contains__(self, asset: Asset) -> bool:
        return asset.symbol in [position.symbol for position in self.positions]

    def __getitem__(self, asset: interfaces.Asset) -> entity.finance.Position | None:
        return next(
            (position for position in self.positions if position.symbol == asset.symbol),
            None,
        )

    def get_position(self, asset: interfaces.Asset) -> entity.finance.Position | None:
        return self[asset]
