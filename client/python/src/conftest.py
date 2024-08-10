import os
import tempfile
from datetime import datetime, timedelta, timezone
from functools import partial
from multiprocessing import get_start_method, set_start_method
from threading import Thread

import pynng
import pytest
import yfinance
from google.protobuf.timestamp_pb2 import Timestamp
from sqlalchemy import Column, DateTime, Integer, String, UniqueConstraint, create_engine, engine, text
from sqlalchemy.orm import declarative_base
from testcontainers.core.container import DockerContainer
from testcontainers.core.network import Network
from testcontainers.core.waiting_utils import wait_for_logs
from testcontainers.minio import MinioContainer
from testcontainers.nats import NatsContainer
from testcontainers.postgres import PostgresContainer

from foreverbull import Order, entity
from foreverbull.entity.finance import OrderStatus
from foreverbull.pb import pb_utils
from foreverbull.pb.finance import finance_pb2
from foreverbull.pb.service import service_pb2


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="function")
def execution(database):
    return entity.backtest.Execution(
        id="test",
        start=datetime(2023, 1, 3, 0, 0, 0, 0, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, 0, 0, 0, 0, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
        benchmark="AAPL",
    )


@pytest.fixture(scope="function")
def namespace_server():
    namespace = dict()

    s = pynng.Rep0(listen="tcp://0.0.0.0:7878")
    s.recv_timeout = 500
    s.send_timeout = 500
    os.environ["NAMESPACE_PORT"] = "7878"

    def runner(s, namespace):
        while True:
            request = service_pb2.NamespaceRequest()
            try:
                request.ParseFromString(s.recv())
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            if request.type == service_pb2.NamespaceRequestType.GET:
                response = service_pb2.NamespaceResponse()
                response.value.update(namespace.get(request.key, {}))
                s.send(response.SerializeToString())
            elif request.type == service_pb2.NamespaceRequestType.SET:
                namespace[request.key] = {k: v for k, v in request.value.items()}
                response = service_pb2.NamespaceResponse()
                s.send(response.SerializeToString())
            else:
                response = service_pb2.NamespaceResponse(error="Invalid task")
                s.send(response.SerializeToString())

    thread = Thread(target=runner, args=(s, namespace))
    thread.start()

    yield namespace

    s.close()
    thread.join()


@pytest.fixture(scope="function")
def parallel_algo_file(spawn_process, execution, database):
    def _process_symbols(server_socket: pynng.Socket) -> list[Order]:
        start = execution.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < execution.end:
            for symbol in execution.symbols:
                pb = finance_pb2.Portfolio(**portfolio.model_dump())
                request = service_pb2.WorkerRequest(
                    task="parallel_algo",
                    timestamp=pb_utils.to_proto_timestamp(start),
                    symbols=[symbol],
                    portfolio=pb,
                )

                server_socket.send(request.SerializeToString())
                response = service_pb2.WorkerResponse()
                response.ParseFromString(server_socket.recv())
                assert response.task == "parallel_algo"
                assert response.HasField("error") is False
                for order in response.orders:
                    orders.append(
                        Order(
                            id=order.id,
                            symbol=order.symbol,
                            amount=order.amount,
                            filled=order.filled,
                            commission=order.commission,
                            limit_price=order.limit_price,
                            stop_price=order.stop_price,
                            created_at=order.created_at.ToDatetime(),
                            status=OrderStatus(order.status),
                        )
                    )
            start += timedelta(days=1)
        return orders

    request = service_pb2.ConfigureExecutionRequest(
        brokerPort=5656,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[],
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5656")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, server_socket=process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def parallel_algo(asset: Asset, portfolio: Portfolio) -> Order:
    return choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)])

Algorithm(
    functions=[
        Function(callable=parallel_algo)
    ]
)
"""
        )
        f.flush()

        yield f.name, request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file(spawn_process, execution, database):
    def _process_symbols(server_socket: pynng.Socket) -> list[Order]:
        start = execution.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < execution.end:
            pb = finance_pb2.Portfolio(**portfolio.model_dump())
            request = service_pb2.WorkerRequest(
                task="non_parallel_algo",
                timestamp=pb_utils.to_proto_timestamp(start),
                symbols=execution.symbols,
                portfolio=pb,
            )

            server_socket.send(request.SerializeToString())
            response = service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "non_parallel_algo"
            assert response.HasField("error") is False
            for order in response.orders:
                orders.append(
                    Order(
                        id=order.id,
                        symbol=order.symbol,
                        amount=order.amount,
                        filled=order.filled,
                        commission=order.commission,
                        limit_price=order.limit_price,
                        stop_price=order.stop_price,
                        created_at=order.created_at.ToDatetime(),
                        status=OrderStatus(order.status),
                    )
                )
            start += timedelta(days=1)
        return orders

    request = service_pb2.ConfigureExecutionRequest(
        brokerPort=5657,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[],
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5657")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, server_socket=process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Assets

def non_parallel_algo(assets: Assets, portfolio: Portfolio) -> list[Order]:
    orders = []
    for asset in assets:
        orders.append(choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)]))
    return orders

Algorithm(
    functions=[
        Function(callable=non_parallel_algo)
    ]
)
"""
        )
        f.flush()
        yield f.name, request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def parallel_algo_file_with_parameters(spawn_process, execution, database):
    def _process_symbols(server_socket) -> list[Order]:
        start = execution.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < execution.end:
            for symbol in execution.symbols:
                pb = finance_pb2.Portfolio(**portfolio.model_dump())
                request = service_pb2.WorkerRequest(
                    task="parallel_algo_with_parameters",
                    timestamp=pb_utils.to_proto_timestamp(start),
                    symbols=[symbol],
                    portfolio=pb,
                )
                server_socket.send(request.SerializeToString())
                response = service_pb2.WorkerResponse()
                response.ParseFromString(server_socket.recv())
                assert response.task == "parallel_algo_with_parameters"
                assert response.HasField("error") is False
                for order in response.orders:
                    orders.append(
                        Order(
                            id=order.id,
                            symbol=order.symbol,
                            amount=order.amount,
                            filled=order.filled,
                            commission=order.commission,
                            limit_price=order.limit_price,
                            stop_price=order.stop_price,
                            created_at=order.created_at.ToDatetime(),
                            status=OrderStatus(order.status),
                        )
                    )

            start += timedelta(days=1)
        return orders

    request = service_pb2.ConfigureExecutionRequest(
        brokerPort=5658,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            service_pb2.ConfigureExecutionRequest.Function(
                name="parallel_algo_with_parameters",
                parameters=[
                    service_pb2.ConfigureExecutionRequest.FunctionParameter(key="low", value="5"),
                    service_pb2.ConfigureExecutionRequest.FunctionParameter(key="high", value="10"),
                ],
            )
        ],
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5658")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, server_socket=process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def parallel_algo_with_parameters(asset: Asset, portfolio: Portfolio, low: int, high: int) -> Order:
    return choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)])

Algorithm(
    functions=[
        Function(callable=parallel_algo_with_parameters)
    ]
)
"""
        )
        f.flush()
        yield f.name, request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file_with_parameters(spawn_process, execution, database):
    def _process_symbols(server_socket) -> list[Order]:
        start = execution.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < execution.end:
            pb = finance_pb2.Portfolio(**portfolio.model_dump())
            request = service_pb2.WorkerRequest(
                task="non_parallel_algo_with_parameters",
                timestamp=pb_utils.to_proto_timestamp(start),
                symbols=execution.symbols,
                portfolio=pb,
            )

            server_socket.send(request.SerializeToString())
            response = service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "non_parallel_algo_with_parameters"
            assert response.HasField("error") is False
            for order in response.orders:
                orders.append(
                    Order(
                        id=order.id,
                        symbol=order.symbol,
                        amount=order.amount,
                        filled=order.filled,
                        commission=order.commission,
                        limit_price=order.limit_price,
                        stop_price=order.stop_price,
                        created_at=order.created_at.ToDatetime(),
                        status=OrderStatus(order.status),
                    )
                )
            start += timedelta(days=1)
        return orders

    request = service_pb2.ConfigureExecutionRequest(
        brokerPort=5659,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            service_pb2.ConfigureExecutionRequest.Function(
                name="non_parallel_algo_with_parameters",
                parameters=[
                    service_pb2.ConfigureExecutionRequest.FunctionParameter(key="low", value="5"),
                    service_pb2.ConfigureExecutionRequest.FunctionParameter(key="high", value="10"),
                ],
            )
        ],
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5659")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, server_socket=process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from random import choice
from foreverbull import Algorithm, Function, Portfolio, Order, Assets

def non_parallel_algo_with_parameters(assets: Assets, portfolio: Portfolio, low: int, high: int) -> list[Order]:
    orders = []
    for asset in assets:
        orders.append(choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10)]))
    return orders

Algorithm(
    functions=[
        Function(callable=non_parallel_algo_with_parameters)
    ]
)
"""
        )
        f.flush()
        yield f.name, request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def multistep_algo_with_namespace(spawn_process, execution, database, namespace_server):
    def _process_symbols(server_socket) -> list[Order]:
        start = execution.start
        portfolio = entity.finance.Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        portfolio = finance_pb2.Portfolio(**portfolio.model_dump())
        orders: list[Order] = []
        while start < execution.end:
            # filter assets
            req = service_pb2.WorkerRequest(
                task="filter_assets",
                timestamp=Timestamp().FromDatetime(start),
                symbols=execution.symbols,
                portfolio=portfolio,
            )
            server_socket.send(req.SerializeToString())
            response = service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "filter_assets"
            assert response.HasField("error") is False

            # measure assets
            for symbol in execution.symbols:
                req = service_pb2.WorkerRequest(
                    task="measure_assets",
                    timestamp=Timestamp().FromDatetime(start),
                    symbols=[symbol],
                    portfolio=portfolio,
                )
                server_socket.send(req.SerializeToString())
                response = service_pb2.WorkerResponse()
                response.ParseFromString(server_socket.recv())
                assert response.task == "measure_assets"
                assert response.HasField("error") is False

            # create orders
            req = service_pb2.WorkerRequest(
                task="create_orders",
                timestamp=Timestamp().FromDatetime(start),
                symbols=execution.symbols,
                portfolio=portfolio,
            )
            server_socket.send(req.SerializeToString())
            response = service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "create_orders"
            assert response.HasField("error") is False
            start += timedelta(days=1)
        return orders

    request = service_pb2.ConfigureExecutionRequest(
        brokerPort=5660,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            service_pb2.ConfigureExecutionRequest.Function(
                name="measure_assets",
                parameters=[
                    service_pb2.ConfigureExecutionRequest.FunctionParameter(key="low", value="5"),
                    service_pb2.ConfigureExecutionRequest.FunctionParameter(key="high", value="10"),
                ],
            ),
            service_pb2.ConfigureExecutionRequest.Function(
                name="create_orders",
                parameters=[],
            ),
            service_pb2.ConfigureExecutionRequest.Function(
                name="filter_assets",
                parameters=[],
            ),
        ],
    )

    process_socket = pynng.Req0(listen="tcp://127.0.0.1:5660")
    process_socket.recv_timeout = 5000
    process_socket.send_timeout = 5000
    _process_symbols = partial(_process_symbols, server_socket=process_socket)

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from foreverbull import Algorithm, Function, Asset, Assets, Portfolio, Order


def measure_assets(asset: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> None:
    pass

def create_orders(assets: Assets, portfolio: Portfolio) -> list[Order]:
    pass

def filter_assets(assets: Assets, portfolio: Portfolio) -> None:
    pass

Algorithm(
    functions=[
        Function(callable=measure_assets),
        Function(callable=create_orders, run_last=True),
        Function(callable=filter_assets, run_first=True),
    ],
    namespaces=["qualified_symbols", "asset_metrics"]
)
"""
        )
        f.flush()
        yield f.name, request, _process_symbols
        process_socket.close()


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

    __table_args__ = (UniqueConstraint("symbol", "time", name="symbol_time_uc"),)


@pytest.fixture(scope="session")
def backtest_entity():
    return entity.backtest.Backtest(
        name="testing_backtest",
        start=datetime(2022, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 12, 29, tzinfo=timezone.utc),
        symbols=[
            "AAPL",
            "AMZN",
            "BAC",
            "BRK-B",
            "CMCSA",
            "CSCO",
            "DIS",
            "GOOG",
            "GOOGL",
            "HD",
            "INTC",
            "JNJ",
            "JPM",
            "KO",
            "MA",
            "META",
            "MRK",
            "MSFT",
            "PEP",
            "PG",
            "TSLA",
            "UNH",
            "V",
            "VZ",
            "WMT",
        ],
    )


@pytest.fixture(scope="session")
def verify_database():
    def _(database: engine.Engine, backtest: entity.backtest.Backtest):
        with database.connect() as conn:
            for symbol in backtest.symbols:
                result = conn.execute(
                    text("SELECT min(time), max(time) FROM ohlc WHERE symbol = :symbol"),
                    {"symbol": symbol},
                )
                res = result.fetchone()
                if res is None:
                    print("FAIL TO FETCH MIN MAX")
                    return False
                start, end = res
                if start is None or end is None:
                    print("START TIME AND TIME IS NONE")
                    return False
                if start.date() != backtest.start.date() or end.date() != backtest.end.date():
                    print("START TIME AND TIME IS NOT EQUAL")
                    print(start.date(), backtest.start.date(), end.date(), backtest.end.date())
                    return False
            return True

    return _


@pytest.fixture(scope="session")
def populate_database():
    def _(database: engine.Engine, backtest: entity.backtest.Backtest):
        with database.connect() as conn:
            for symbol in backtest.symbols:
                feed = yfinance.Ticker(symbol)
                info = feed.info
                asset = entity.finance.Asset(
                    symbol=info["symbol"],
                    name=info["longName"],
                    title=info["shortName"],
                    asset_type=info["quoteType"],
                )
                conn.execute(
                    text(
                        """INSERT INTO asset (symbol, name, title, asset_type)
                        VALUES (:symbol, :name, :title, :asset_type) ON CONFLICT DO NOTHING"""
                    ),
                    {"symbol": asset.symbol, "name": asset.name, "title": asset.title, "asset_type": asset.asset_type},
                )
                data = feed.history(start=backtest.start, end=backtest.end + timedelta(days=1))
                for idx, row in data.iterrows():
                    time = datetime(idx.year, idx.month, idx.day, idx.hour, idx.minute, idx.second)
                    ohlc = entity.finance.OHLC(
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
                            VALUES (:symbol, :open, :high, :low, :close, :volume, :time) ON CONFLICT DO NOTHING"""
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

    return _


@pytest.fixture(scope="session")
def database(backtest_entity: entity.backtest.Backtest, verify_database, populate_database):
    postgres = PostgresContainer("postgres:alpine")
    postgres = postgres.with_volume_mapping("postgres_data", "/var/lib/postgresql/data", mode="rw")
    with postgres as postgres:
        engine = create_engine(postgres.get_connection_url())
        Base.metadata.create_all(engine)
        os.environ["DATABASE_URL"] = postgres.get_connection_url()
        if not verify_database(engine, backtest_entity):
            populate_database(engine, backtest_entity)
        yield engine


@pytest.fixture(scope="session")
def local_environment(network, minio, nats, postgres):
    with Network() as network:
        postgres = PostgresContainer(
            "postgres:13.3-alpine",
            network=network.name,
            hostname="postgres",
            username="postgres",
            password="postgres",
            dbname="postgres",
        )
        minio = MinioContainer("minio/minio:latest", network=network.name, hostname="minio")
        nats = NatsContainer("nats:2.10-alpine", network=network.name, hostname="nats")
        nats = nats.with_command("-js")
        with postgres as postgres, minio as minio, nats as nats:
            foreverbull = DockerContainer(os.environ.get("BROKER_IMAGE", ""))
            foreverbull.with_network(network)
            foreverbull.with_env("POSTGRES_URL", "postgres://postgres:postgres@postgres:5432/postgres")
            foreverbull.with_env("NATS_URL", "nats://nats:4222")
            foreverbull.with_env("MINIO_URL", "minio:9000")
            foreverbull.with_env("BACKTEST_IMAGE", os.environ.get("BACKTEST_IMAGE", "lhjnilsson/zipline:latest"))
            foreverbull.with_volume_mapping("/var/run/docker.sock", "/var/run/docker.sock", mode="rw")
            with foreverbull as foreverbull:
                wait_for_logs(foreverbull, "RUNNING", 10)
                yield foreverbull
