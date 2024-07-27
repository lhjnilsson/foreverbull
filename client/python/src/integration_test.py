import os
import time
from datetime import datetime, timezone
from functools import partial

import pandas as pd
import pynng
import pytest
from zipline.api import order_target, symbol
from zipline.data import bundles
from zipline.data.bundles import register
from zipline.errors import SymbolNotFound
from zipline.utils.calendar_utils import get_calendar
from zipline.utils.run_algo import BenchmarkSpec, _run

from foreverbull import Foreverbull, entity
from foreverbull.pb_gen import backtest_pb2, finance_pb2, service_pb2
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.entity import Period
from foreverbull_zipline.execution import Execution


@pytest.fixture(scope="session")
def execution(spawn_process) -> entity.backtest.Execution:
    return entity.backtest.Execution(
        calendar="XNYS",
        start=datetime(2022, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 12, 29, tzinfo=timezone.utc),
        benchmark=None,
        # Top 25 largest companies on sp500
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
def foreverbull_bundle(execution: entity.backtest.Execution, database):
    backtest_entity = entity.backtest.Backtest(
        name="test_backtest",
        calendar=execution.calendar,
        start=execution.start,
        end=execution.end,
        symbols=execution.symbols,
    )

    def sanity_check(bundle):
        for s in backtest_entity.symbols:
            try:
                stored_asset = bundle.asset_finder.lookup_symbol(s, as_of_date=None)
            except SymbolNotFound:
                raise LookupError(f"Asset {s} not found in bundle")
            backtest_start = pd.Timestamp(backtest_entity.start).normalize().tz_localize(None)
            if backtest_start < stored_asset.start_date:
                print("Start date is not correct", backtest_start, stored_asset.start_date)
                raise ValueError("Start date is not correct")

            backtest_end = pd.Timestamp(backtest_entity.end).normalize().tz_localize(None)
            if backtest_end > stored_asset.end_date:
                print("End date is not correct", backtest_end, stored_asset.end_date)
                raise ValueError("End date is not correct")

    bundles.register("foreverbull", SQLIngester(), calendar_name=execution.calendar)
    try:
        print("Loading bundle")
        sanity_check(bundles.load("foreverbull", os.environ, None))
    except (ValueError, LookupError) as exc:
        print("Creating bundle", exc)
        exc = Execution()
        exc._ingest(backtest_entity)


def baseline_performance_initialize(context):
    context.i = 0
    context.held_positions = []


def baseline_performance_handle_data(context, data, execution: entity.backtest.Execution):
    context.i += 1
    if context.i < 30:
        return

    for s in execution.symbols:
        short_mean = data.history(symbol(s), "close", bar_count=10, frequency="1d").mean()
        long_mean = data.history(symbol(s), "close", bar_count=30, frequency="1d").mean()
        if short_mean > long_mean and s not in context.held_positions:
            order_target(symbol(s), 10)
            context.held_positions.append(s)
        elif short_mean < long_mean and s in context.held_positions:
            order_target(symbol(s), 0)
            context.held_positions.remove(s)


@pytest.fixture(scope="session")
def baseline_performance(foreverbull_bundle, execution):
    register("foreverbull", SQLIngester(), calendar_name="XNYS")
    benchmark_spec = BenchmarkSpec.from_cli_params(
        no_benchmark=True,
        benchmark_sid=None,
        benchmark_symbol=None,
        benchmark_file=None,
    )
    if os.path.exists("baseline_performance.pickle"):
        os.remove("baseline_performance.pickle")

    trading_calendar = get_calendar("XNYS")
    _run(
        initialize=baseline_performance_initialize,
        handle_data=partial(baseline_performance_handle_data, execution=execution),
        before_trading_start=None,
        analyze=None,
        algofile=None,
        algotext=None,
        defines=None,
        data_frequency="daily",
        capital_base=100000,
        bundle="foreverbull",
        bundle_timestamp=pd.Timestamp.utcnow(),
        start=pd.Timestamp(execution.start).normalize().tz_localize(None),
        end=pd.Timestamp(execution.end).normalize().tz_localize(None),
        output="baseline_performance.pickle",
        trading_calendar=trading_calendar,
        print_algo=False,
        metrics_set="default",
        local_namespace=None,
        environ=os.environ,
        blotter="default",
        benchmark_spec=benchmark_spec,
        custom_loader=None,
    )

    return pd.read_pickle("baseline_performance.pickle").reset_index(drop=True)


@pytest.fixture(scope="session")
def zipline_socket():
    execution = Execution(port=6666)
    execution.start()
    for _ in range(10):
        try:
            pynng_socket = pynng.Req0(
                dial=f"tcp://{execution.socket_config.host}:{execution.socket_config.port}",
                block_on_dial=True,
            )
            pynng_socket.recv_timeout = 10000
            pynng_socket.send_timeout = 10000
            break
        except pynng.exceptions.ConnectionRefused:
            time.sleep(0.1)
    else:
        raise Exception("Failed to connect to execution socket")

    def run(
        execution: entity.backtest.Execution,
    ):
        request = service_pb2.Message(task="configure_execution")
        request.data.update(execution.model_dump())
        pynng_socket.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(pynng_socket.recv())
        if response.HasField("error"):
            raise Exception(response.error)
        request = service_pb2.Message(task="run_execution")
        pynng_socket.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(pynng_socket.recv())
        if response.HasField("error"):
            raise Exception(response.error)
        return pynng_socket

    yield run
    request = service_pb2.Message(task="stop")
    pynng_socket.send(request.SerializeToString())
    pynng_socket.recv()
    pynng_socket.close()
    execution.join()


@pytest.mark.parametrize("file_path", ["examples/parallel.py"])
def test_integration(zipline_socket, execution, foreverbull_bundle, baseline_performance, file_path):
    broker_socket = pynng.Req0(listen="tcp://0.0.0.0:8888")
    broker_socket.recv_timeout = 10000
    broker_socket.send_timeout = 10000

    namespace_socket = pynng.Rep0(listen="tcp://0.0.0.0:9999")
    namespace_socket.recv_timeout = 10000
    namespace_socket.send_timeout = 10000

    service_instance = entity.service.Instance(
        id="test_instance",
        broker_port=8888,
        namespace_port=9999,
        database_url=os.environ["DATABASE_URL"],
        functions={"handle_data": {"parameters": {}}},
    )

    with Foreverbull(file_path):
        backtest = zipline_socket(execution)
        foreverbull_socket = pynng.Req0(dial="tcp://127.0.0.1:5555")
        foreverbull_socket.recv_timeout = 10000
        foreverbull_socket.send_timeout = 10000

        request = service_pb2.Message(task="configure_execution")
        request.data.update(service_instance.model_dump())
        foreverbull_socket.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(foreverbull_socket.recv())
        assert response.HasField("error") is False
        request = service_pb2.Message(task="run_execution")
        foreverbull_socket.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(foreverbull_socket.recv())
        assert response.HasField("error") is False

        while True:
            request = service_pb2.Message(task="get_period")
            backtest.send(request.SerializeToString())
            period = backtest_pb2.Period()
            period.ParseFromString(backtest.recv())

            portfolio = finance_pb2.Portfolio(
                cash=period.starting_cash,
                value=period.portfolio_value,
                positions=period.positions,
            )

            request = service_pb2.Request(
                task="handle_data",
                timestamp=period.timestamp,
                symbols=execution.symbols,
                portfolio=portfolio,
            )
            broker_socket.send(request.SerializeToString())
            response = service_pb2.Request()
            response.ParseFromString(broker_socket.recv())
            if response.HasField("error"):
                raise Exception(response.error)
            for o in response.orders:
                order = entity.finance.Order(
                    symbol=o.symbol,
                    amount=o.amount,
                    limit_price=o.limit_price,
                    stop_price=o.stop_price,
                )
                order_request = service_pb2.Message(
                    task="place_order",
                )
                order_request.data.update(order.model_dump())
                backtest.send(order_request.SerializeToString())
                response = service_pb2.Message()
                response.ParseFromString(backtest.recv())
                if response.HasField("error"):
                    raise Exception(response.error)

            request = service_pb2.Message(task="continue")
            backtest.send(request.SerializeToString())
            response = service_pb2.Message()
            response.ParseFromString(backtest.recv())
            if response.HasField("error"):
                raise Exception(response.error)

        request = service_pb2.Message(task="get_execution_result")
        backtest.send(request.SerializeToString())
        response = service_pb2.Message()
        response.ParseFromString(backtest.recv())
        assert response.HasField("error") is False
        result = pd.DataFrame(response.data["periods"]).reset_index(drop=True)
        result = result.drop(columns=["timestamp"])
        baseline_performance = baseline_performance[result.columns]
        assert baseline_performance.equals(result)

    broker_socket.close()
    namespace_socket.close()
