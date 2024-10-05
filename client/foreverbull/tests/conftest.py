import importlib.util
import os
import tempfile
from datetime import datetime, timedelta, timezone
from functools import partial
from multiprocessing import get_start_method, set_start_method
from threading import Thread

import pynng
import pytest
from foreverbull import Algorithm, Function, Order, models
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import backtest_pb2, execution_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull.pb.foreverbull.service import worker_pb2, worker_service_pb2


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="session")
def backtest_entity():
    return backtest_pb2.Backtest(
        name="testing_backtest",
        start_date=common_pb2.Date(year=2022, month=1, day=3),
        end_date=common_pb2.Date(year=2023, month=12, day=29),
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


@pytest.fixture(scope="function")
def execution(fb_database):
    return execution_pb2.Execution(
        id="test",
        start_date=common_pb2.Date(year=2022, month=1, day=3),
        end_date=common_pb2.Date(year=2023, month=12, day=29),
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
    spec = importlib.util.spec_from_file_location(
        "",
        file_path,
    )
    if spec is None or spec.loader is None:
        raise Exception(f"Failed to load {file_path}")
    source = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(source)
    return Algorithm(
        [Function(callable=models.Algorithm._functions["parallel_algo"]["callable"])],
        models.Algorithm._namespaces,
    )


@pytest.fixture(scope="function")
def parallel_algo_file(spawn_process, execution: execution_pb2.Execution, fb_database):
    def _process_symbols(server_socket: pynng.Socket) -> list[Order]:
        start = pb_utils.from_proto_date_to_pydate(execution.start_date)
        portfolio = finance_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < pb_utils.from_proto_date_to_pydate(execution.end_date):
            for symbol in execution.symbols:
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
            start += timedelta(days=1)
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

        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file(
    spawn_process, execution: execution_pb2.Execution, fb_database
):
    def _process_symbols(server_socket: pynng.Socket) -> list[Order]:
        start = pb_utils.from_proto_date_to_pydate(execution.start_date)
        portfolio = finance_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < pb_utils.from_proto_date_to_pydate(execution.end_date):
            request = worker_service_pb2.WorkerRequest(
                task="non_parallel_algo",
                symbols=execution.symbols,
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
            start += timedelta(days=1)
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
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def parallel_algo_file_with_parameters(
    spawn_process, execution: execution_pb2.Execution, fb_database
):
    def _process_symbols(server_socket) -> list[Order]:
        start = pb_utils.from_proto_date_to_pydate(execution.start_date)
        portfolio = finance_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < pb_utils.from_proto_date_to_pydate(execution.end_date):
            for symbol in execution.symbols:
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

            start += timedelta(days=1)
        return orders

    request = worker_pb2.ExecutionConfiguration(
        brokerPort=5658,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            worker_pb2.ExecutionConfiguration.Function(
                name="parallel_algo_with_parameters",
                parameters=[
                    worker_pb2.ExecutionConfiguration.FunctionParameter(
                        key="low", value="5"
                    ),
                    worker_pb2.ExecutionConfiguration.FunctionParameter(
                        key="high", value="10"
                    ),
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
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def non_parallel_algo_file_with_parameters(
    spawn_process, execution: execution_pb2.Execution, fb_database
):
    def _process_symbols(server_socket) -> list[Order]:
        start = pb_utils.from_proto_date_to_pydate(execution.start_date)
        portfolio = finance_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < pb_utils.from_proto_date_to_pydate(execution.end_date):
            request = worker_service_pb2.WorkerRequest(
                task="non_parallel_algo_with_parameters",
                symbols=execution.symbols,
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
            start += timedelta(days=1)
        return orders

    request = worker_pb2.ExecutionConfiguration(
        brokerPort=5659,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            worker_pb2.ExecutionConfiguration.Function(
                name="non_parallel_algo_with_parameters",
                parameters=[
                    worker_pb2.ExecutionConfiguration.FunctionParameter(
                        key="low", value="5"
                    ),
                    worker_pb2.ExecutionConfiguration.FunctionParameter(
                        key="high", value="10"
                    ),
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
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()


@pytest.fixture(scope="function")
def multistep_algo_with_namespace(
    spawn_process, execution: execution_pb2.Execution, fb_database, namespace_server
):
    def _process_symbols(server_socket) -> list[Order]:
        start = pb_utils.from_proto_date_to_pydate(execution.start_date)
        portfolio = finance_pb2.Portfolio(
            cash=0,
            portfolio_value=0,
            positions=[],
        )
        orders: list[Order] = []
        while start < pb_utils.from_proto_date_to_pydate(execution.end_date):
            req = worker_service_pb2.WorkerRequest(
                task="filter_assets",
                symbols=execution.symbols,
                portfolio=portfolio,
            )
            server_socket.send(req.SerializeToString())
            response = worker_service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "filter_assets"
            assert response.HasField("error") is False

            # measure assets
            for symbol in execution.symbols:
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
                symbols=execution.symbols,
                portfolio=portfolio,
            )
            server_socket.send(req.SerializeToString())
            response = worker_service_pb2.WorkerResponse()
            response.ParseFromString(server_socket.recv())
            assert response.task == "create_orders"
            assert response.HasField("error") is False
            start += timedelta(days=1)
        return orders

    request = worker_pb2.ExecutionConfiguration(
        brokerPort=5660,
        namespacePort=7878,
        databaseURL=os.environ["DATABASE_URL"],
        functions=[
            worker_pb2.ExecutionConfiguration.Function(
                name="measure_assets",
                parameters=[
                    worker_pb2.ExecutionConfiguration.FunctionParameter(
                        key="low", value="5"
                    ),
                    worker_pb2.ExecutionConfiguration.FunctionParameter(
                        key="high", value="10"
                    ),
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
        yield get_algo(f.name), request, _process_symbols
        process_socket.close()
