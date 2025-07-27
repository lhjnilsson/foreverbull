import os
import tempfile

from datetime import timedelta
from functools import partial
from multiprocessing import get_start_method
from multiprocessing import set_start_method
from threading import Thread
from typing import Iterator

import pynng
import pytest

from google.protobuf.timestamp_pb2 import Timestamp

from foreverbull import Algorithm
from foreverbull import Order
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb2 as worker_service_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="session")
def backtest_entity():
    return backtest_pb2.Backtest(
        name="testing_backtest",
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=common_pb2.Date(year=2022, month=1, day=3),
                end_date=common_pb2.Date(year=2023, month=12, day=29),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AAPL",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="AMZN",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="BAC",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="BRK-B",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="CMCSA",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="CSCO",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="DIS",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="GOOG",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="GOOGL",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="HD",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="INTC",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="JNJ",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="JPM",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="KO",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MA",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="META",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MRK",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="MSFT",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="PEP",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="PG",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="TSLA",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="UNH",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="V",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="VZ",
                        enabled=True,
                    ),
                    backtest_pb2.Backtest.Period.Asset(
                        symbol="WMT",
                        enabled=True,
                    ),
                ],
            )
        ],
    )


@pytest.fixture(scope="function")
def execution(fb_database):
    return execution_pb2.Execution(
        id="test",
        start_date=common_pb2.Date(year=2022, month=1, day=3),
        end_date=common_pb2.Date(year=2023, month=12, day=29),
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
            request = worker_service_pb2.NamespaceRequest()
            try:
                request.ParseFromString(s.recv())
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            if request.type == worker_service_pb2.NamespaceRequestType.GET:
                response = worker_service_pb2.NamespaceResponse()
                response.value.update(namespace.get(request.key, {}))
            elif request.type == worker_service_pb2.NamespaceRequestType.SET:
                namespace[request.key] = {k: v for k, v in request.value.items()}
                response = worker_service_pb2.NamespaceResponse()
                response.value.update(namespace[request.key])
            else:
                response = worker_service_pb2.NamespaceResponse(error="Invalid task")
            s.send(response.SerializeToString())

    thread = Thread(target=runner, args=(s, namespace))
    thread.start()

    yield namespace

    s.close()
    thread.join()


def get_algo(file_path: str) -> Algorithm:
    return Algorithm.from_file_path(file_path)


def backtest_date_range(backtest: backtest_pb2.Backtest) -> Iterator[tuple[Timestamp, list[str]]]:
    for period in backtest.periods:
        start = pb_utils.from_proto_date_to_pydate(period.start_date)
        end = pb_utils.from_proto_date_to_pydate(period.end_date)
        symbols = [asset.symbol for asset in period.assets if asset.enabled]
        while start <= end:
            yield Timestamp(), symbols
            start += timedelta(days=1)


@pytest.fixture(scope="function")
def parallel_algo_file(spawn_process, backtest_entity: backtest_pb2.Backtest, fb_database):
    def _process_symbols(server_socket: pynng.Socket) -> list[Order]:
        portfolio = marketdata_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        for ts, symbols in backtest_date_range(backtest_entity):
            portfolio.timestamp = ts
            for symbol in symbols:
                request = worker_service_pb2.WorkerRequest(
                    task="parallel_algo",
                    symbols=[symbol],
                    portfolio=portfolio,
                )

                server_socket.send(request.SerializeToString())
                response = worker_service_pb2.WorkerResponse()
                response.ParseFromString(server_socket.recv())
                assert response.task == "parallel_algo"
                assert response.HasField("error") is False
                for order in response.orders:
                    orders.append(
                        Order(
                            symbol=order.symbol,
                            amount=order.amount,
                        )
                    )
        return orders

    request = worker_pb2.ExecutionConfiguration(
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
import random
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def parallel_algo(asset: Asset, portfolio: Portfolio) -> Order:
    if random.randint(0, 1):
        portfolio.order(asset.symbol, 10)
    else:
        portfolio.order(asset.symbol, -10)

algo = Algorithm(
    functions=[
        Function(callable=parallel_algo)
    ]
)
"""
        )
        f.flush()

        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file(spawn_process, backtest_entity: backtest_pb2.Backtest, fb_database):
    def _process_symbols(server_socket: pynng.Socket) -> list[Order]:
        portfolio = marketdata_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        for ts, symbols in backtest_date_range(backtest_entity):
            portfolio.timestamp = ts
            request = worker_service_pb2.WorkerRequest(
                task="non_parallel_algo",
                symbols=symbols,
                portfolio=portfolio,
            )

            server_socket.send(request.SerializeToString())
            response = worker_service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "non_parallel_algo"
            assert response.HasField("error") is False
            for order in response.orders:
                orders.append(
                    Order(
                        symbol=order.symbol,
                        amount=order.amount,
                    )
                )
        return orders

    request = worker_pb2.ExecutionConfiguration(
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
import random
from foreverbull import Algorithm, Function, Portfolio, Order, Assets

def non_parallel_algo(assets: Assets, portfolio: Portfolio) -> list[Order]:
    for asset in assets:
        if random.randint(0, 1):
            portfolio.order(asset.symbol, 10)
        else:
            portfolio.order(asset.symbol, -10)

algo = Algorithm(
    functions=[
        Function(callable=non_parallel_algo)
    ]
)
"""
        )
        f.flush()
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def parallel_algo_file_with_parameters(spawn_process, backtest_entity: backtest_pb2.Backtest, fb_database):
    def _process_symbols(server_socket) -> list[Order]:
        portfolio = marketdata_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        for ts, symbols in backtest_date_range(backtest_entity):
            portfolio.timestamp = ts
            for symbol in symbols:
                request = worker_service_pb2.WorkerRequest(
                    task="parallel_algo_with_parameters",
                    symbols=[symbol],
                    portfolio=portfolio,
                )
                server_socket.send(request.SerializeToString())
                response = worker_service_pb2.WorkerResponse()
                response.ParseFromString(server_socket.recv())
                assert response.task == "parallel_algo_with_parameters"
                assert response.HasField("error") is False
                for order in response.orders:
                    orders.append(
                        Order(
                            symbol=order.symbol,
                            amount=order.amount,
                        )
                    )

        return orders

    request = worker_pb2.ExecutionConfiguration(
        brokerPort=5658,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            worker_pb2.ExecutionConfiguration.Function(
                name="parallel_algo_with_parameters",
                parameters=[
                    worker_pb2.ExecutionConfiguration.FunctionParameter(key="low", value="5"),
                    worker_pb2.ExecutionConfiguration.FunctionParameter(key="high", value="10"),
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
import random
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def parallel_algo_with_parameters(asset: Asset, portfolio: Portfolio, low: int, high: int) -> Order:
    if random.randint(0, 1):
        portfolio.order(asset.symbol, 10)
    else:
        portfolio.order(asset.symbol, -10)

algo = Algorithm(
    functions=[
        Function(callable=parallel_algo_with_parameters)
    ]
)
"""
        )
        f.flush()
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file_with_parameters(spawn_process, backtest_entity: backtest_pb2.Backtest, fb_database):
    def _process_symbols(server_socket) -> list[Order]:
        portfolio = marketdata_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        for ts, symbols in backtest_date_range(backtest_entity):
            portfolio.timestamp = ts
            request = worker_service_pb2.WorkerRequest(
                task="non_parallel_algo_with_parameters",
                symbols=symbols,
                portfolio=portfolio,
            )

            server_socket.send(request.SerializeToString())
            response = worker_service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "non_parallel_algo_with_parameters"
            assert response.HasField("error") is False
            for order in response.orders:
                orders.append(
                    Order(
                        symbol=order.symbol,
                        amount=order.amount,
                    )
                )
        return orders

    request = worker_pb2.ExecutionConfiguration(
        brokerPort=5659,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            worker_pb2.ExecutionConfiguration.Function(
                name="non_parallel_algo_with_parameters",
                parameters=[
                    worker_pb2.ExecutionConfiguration.FunctionParameter(key="low", value="5"),
                    worker_pb2.ExecutionConfiguration.FunctionParameter(key="high", value="10"),
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
import random
from foreverbull import Algorithm, Function, Portfolio, Order, Assets

def non_parallel_algo_with_parameters(assets: Assets, portfolio: Portfolio, low: int, high: int) -> list[Order]:
    for asset in assets:
        if random.randint(0, 1):
            portfolio.order(asset.symbol, 10)
        else:
            portfolio.order(asset.symbol, -10)

algo = Algorithm(
    functions=[
        Function(callable=non_parallel_algo_with_parameters)
    ]
)
"""
        )
        f.flush()
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def multistep_algo_with_namespace(spawn_process, backtest_entity: backtest_pb2.Backtest, fb_database, namespace_server):
    def _process_symbols(server_socket) -> list[Order]:
        portfolio = marketdata_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        for ts, symbols in backtest_date_range(backtest_entity):
            portfolio.timestamp = ts
            req = worker_service_pb2.WorkerRequest(
                task="filter_assets",
                symbols=symbols,
                portfolio=portfolio,
            )
            server_socket.send(req.SerializeToString())
            response = worker_service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "filter_assets"
            assert response.HasField("error") is False

            # measure assets
            for symbol in symbols:
                req = worker_service_pb2.WorkerRequest(
                    task="measure_assets",
                    symbols=[symbol],
                    portfolio=portfolio,
                )
                server_socket.send(req.SerializeToString())
                response = worker_service_pb2.WorkerResponse()
                response.ParseFromString(server_socket.recv())
                assert response.task == "measure_assets"
                assert response.HasField("error") is False

            # create orders
            req = worker_service_pb2.WorkerRequest(
                task="create_orders",
                symbols=symbols,
                portfolio=portfolio,
            )
            server_socket.send(req.SerializeToString())
            response = worker_service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "create_orders"
            assert response.HasField("error") is False
        return orders

    request = worker_pb2.ExecutionConfiguration(
        brokerPort=5660,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            worker_pb2.ExecutionConfiguration.Function(
                name="measure_assets",
                parameters=[
                    worker_pb2.ExecutionConfiguration.FunctionParameter(key="low", value="5"),
                    worker_pb2.ExecutionConfiguration.FunctionParameter(key="high", value="10"),
                ],
            ),
            worker_pb2.ExecutionConfiguration.Function(
                name="create_orders",
                parameters=[],
            ),
            worker_pb2.ExecutionConfiguration.Function(
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

algo = Algorithm(
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
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()
