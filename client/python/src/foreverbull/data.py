import logging
import os
import re
from contextlib import contextmanager
from datetime import datetime

import pynng
from pandas import DataFrame, read_sql_query
from sqlalchemy import create_engine, engine

from foreverbull import socket


# Hacky way to get the database URL, TODO: find a better way
def get_engine(url: str):
    log = logging.getLogger(__name__)

    if url.startswith("postgres://"):
        url = url.replace("postgres://", "postgresql://", 1)

    try:
        engine = create_engine(url)
        engine.connect()
        return engine
    except Exception as e:
        log.warning(f"Could not connect to {url}: {e}")

    for hostname in ["localhost", "postgres", "127.0.0.1"]:
        try:
            database_port = re.search(r":(\d+)/", url).group(1)
            url = url.replace(f":{database_port}", ":5432", 1)
            database_host = re.search(r"@([^/]+):", url).group(1)
            url = url.replace(f"@{database_host}:", f"@{hostname}:", 1)
            engine = create_engine(url)
            engine.connect()
            return engine
        except Exception as e:
            log.warning(f"Could not connect to {url}: {e}")
    raise Exception("Could not connect to database")


@contextmanager
def namespace_socket() -> pynng.Socket:
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

    def __getattr__(self, name: str) -> any:
        with namespace_socket() as s:
            request = socket.Request(task=f"get:{name}")
            s.send(request.serialize())
            response = socket.Response.deserialize(s.recv())
            if response.error:
                raise Exception(response.error)
            return response.data[self._symbol]

    def __setattr__(self, name: str, value: any) -> None:
        if name.startswith("_"):
            super().__setattr__(name, value)
            return
        with namespace_socket() as s:
            request = socket.Request(task=f"set:{name}", data={self._symbol: value})
            s.send(request.serialize())
            response = socket.Response.deserialize(s.recv())
            if response.error:
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

    def __getattr__(self, name: str) -> any:
        with namespace_socket() as s:
            request = socket.Request(task=f"get:{name}")
            s.send(request.serialize())
            response = socket.Response.deserialize(s.recv())
            if response.error:
                raise Exception(response.error)
            return response.data

    def __setattr__(self, name: str, value: any) -> None:
        if name.startswith("_"):
            super().__setattr__(name, value)
            return
        with namespace_socket() as s:
            request = socket.Request(task=f"set:{name}", data=value)
            s.send(request.serialize())
            response = socket.Response.deserialize(s.recv())
            if response.error:
                raise Exception(response.error)
            return None

    @property
    def symbols(self):
        return self._symbols

    def __iter__(self):
        for symbol in self.symbols:
            yield Asset(self._as_of, self._db, symbol)
