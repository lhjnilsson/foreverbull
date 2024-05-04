import os
import tempfile
from datetime import datetime, timedelta, timezone
from functools import partial
from multiprocessing import get_start_method, set_start_method

import pynng
import pytest
import yfinance
from sqlalchemy import Column, DateTime, Integer, String, create_engine, engine, text
from sqlalchemy.orm import declarative_base
from testcontainers.postgres import PostgresContainer

from foreverbull import Order, entity, socket
from foreverbull_zipline.entity import IngestConfig


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="function")
def execution(database):
    return entity.backtest.Execution(
        id="test",
        calendar="NYSE",
        start=datetime(2023, 1, 1, 0, 0, 0, 0, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, 0, 0, 0, 0, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
        benchmark="AAPL",
    )


@pytest.fixture(scope="function")
def parallel_algo_file(ingest_config, database):
    def _process_symbols(server_socket):
        start = ingest_config.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders = []
        while start < ingest_config.end:
            for symbol in ingest_config.symbols:
                req = entity.service.Request(
                    timestamp=start,
                    symbols=[symbol],
                    portfolio=portfolio,
                )
                server_socket.send(socket.Request(task="handle_data", data=req).serialize())
                response = socket.Response.deserialize(server_socket.recv())
                assert response.task == "handle_data"
                assert response.error is None
                assert response.data
                for order in response.data:
                    orders.append(Order(**order))
            start += timedelta(days=1)
        return orders

    instance = entity.service.Instance(
        id="test",
        broker_port=5656,
        database_url=os.environ["DATABASE_URL"],
        functions={"handle_data": entity.service.Instance.Parameter(parameters={})},
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5656")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def handle_data(asset: Asset, portfolio: Portfolio) -> Order:
    return choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)])
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
        )
        f.flush()

        yield f.name, instance, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file(ingest_config, database):
    def _process_symbols(server_socket):
        start = ingest_config.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders = []
        while start < ingest_config.end:
            req = entity.service.Request(
                timestamp=start,
                symbols=ingest_config.symbols,
                portfolio=portfolio,
            )
            server_socket.send(socket.Request(task="handle_data", data=req).serialize())
            response = socket.Response.deserialize(server_socket.recv())
            assert response.task == "handle_data"
            assert response.error is None
            assert response.data
            for order in response.data:
                orders.append(Order(**order))

            start += timedelta(days=1)
        return orders

    instance = entity.service.Instance(
        id="test",
        broker_port=5656,
        database_url=os.environ["DATABASE_URL"],
        functions={
            "handle_data": entity.service.Instance.Parameter(
                parameters={},
            )
        },
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5656")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Assets

def handle_data(assets: Assets, portfolio: Portfolio) -> list[Order]:
    orders = []
    for asset in assets:
        orders.append(choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)]))
    return orders

Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
        )
        f.flush()
        yield f.name, instance, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def parallel_algo_file_with_parameters(ingest_config, database):
    def _process_symbols(server_socket):
        start = ingest_config.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders = []
        while start < ingest_config.end:
            for symbol in ingest_config.symbols:
                req = entity.service.Request(
                    timestamp=start,
                    symbols=[symbol],
                    portfolio=portfolio,
                )
                server_socket.send(socket.Request(task="handle_data", data=req).serialize())
                response = socket.Response.deserialize(server_socket.recv())
                assert response.task == "handle_data"
                assert response.error is None
                assert response.data
                for order in response.data:
                    orders.append(Order(**order))

            start += timedelta(days=1)
        return orders

    instance = entity.service.Instance(
        id="test",
        broker_port=5656,
        database_url=os.environ["DATABASE_URL"],
        functions={
            "handle_data": entity.service.Instance.Parameter(
                parameters={
                    "low": "15",
                    "high": "25",
                }
            )
        },
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5656")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def handle_data(asset: Asset, portfolio: Portfolio, low: int, high: int) -> Order:
    return choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)])
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
        )
        f.flush()
        yield f.name, instance, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file_with_parameters(ingest_config, database):
    def _process_symbols(server_socket):
        start = ingest_config.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders = []
        while start < ingest_config.end:
            req = entity.service.Request(
                timestamp=start,
                symbols=ingest_config.symbols,
                portfolio=portfolio,
            )
            server_socket.send(socket.Request(task="handle_data", data=req).serialize())
            response = socket.Response.deserialize(server_socket.recv())
            assert response.task == "handle_data"
            assert response.error is None
            assert response.data
            for order in response.data:
                orders.append(Order(**order))
            start += timedelta(days=1)
        return orders

    instance = entity.service.Instance(
        id="test",
        broker_port=5656,
        database_url=os.environ["DATABASE_URL"],
        functions={
            "handle_data": entity.service.Instance.Parameter(
                parameters={
                    "low": "15",
                    "high": "25",
                },
            )
        },
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5656")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Assets

def handle_data(assets: Assets, portfolio: Portfolio, low: int, high: int) -> list[Order]:
    orders = []
    for asset in assets:
        orders.append(choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)]))
    return orders
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
        )
        f.flush()
        yield f.name, instance, _process_symbols
        process_socket.close()


@pytest.fixture(scope="session")
def ingest_config():
    return IngestConfig(
        calendar="NYSE",
        start=datetime(2023, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
    )


Base = declarative_base()


class Asset(Base):
    __tablename__ = "asset"
    symbol = Column("symbol", String(), primary_key=True)
    name = Column("name", String())
    title = Column("title", String())
    asset_type = Column("asset_type", String())


class OHLC(Base):
    __tablename__ = "ohlc"
    id = Column(Integer, primary_key=True)
    symbol = Column(String())
    open = Column(Integer())
    high = Column(Integer())
    low = Column(Integer())
    close = Column(Integer())
    volume = Column(Integer())
    time = Column(DateTime())


@pytest.fixture(scope="session")
def database(ingest_config):
    with PostgresContainer("postgres:alpine") as postgres:
        engine = create_engine(postgres.get_connection_url())
        Base.metadata.create_all(engine)
        populate_database(engine, ingest_config)
        os.environ["DATABASE_URL"] = postgres.get_connection_url()
        yield engine


def populate_database(database: engine.Engine, ic: IngestConfig):
    with database.connect() as conn:
        for symbol in ic.symbols:
            feed = yfinance.Ticker(symbol)
            info = feed.info
            asset = Asset(
                symbol=info["symbol"], name=info["longName"], title=info["shortName"], asset_type=info["quoteType"]
            )
            conn.execute(
                text(
                    """INSERT INTO asset (symbol, name, title, asset_type) 
                    VALUES (:symbol, :name, :title, :asset_type)"""
                ),
                {"symbol": asset.symbol, "name": asset.name, "title": asset.title, "asset_type": asset.asset_type},
            )
            data = feed.history(start=ic.start, end=ic.end + timedelta(days=1))
            for idx, row in data.iterrows():
                time = datetime(idx.year, idx.month, idx.day, idx.hour, idx.minute, idx.second)
                ohlc = OHLC(
                    symbol=symbol,
                    open=row.Open,
                    high=row.High,
                    low=row.Low,
                    close=row.Close,
                    volume=row.Volume,
                    time=time,
                )
                conn.execute(
                    text(
                        """INSERT INTO ohlc (symbol, open, high, low, close, volume, time) 
                        VALUES (:symbol, :open, :high, :low, :close, :volume, :time)"""
                    ),
                    {
                        "symbol": ohlc.symbol,
                        "open": ohlc.open,
                        "high": ohlc.high,
                        "low": ohlc.low,
                        "close": ohlc.close,
                        "volume": ohlc.volume,
                        "time": ohlc.time,
                    },
                )
        conn.commit()
