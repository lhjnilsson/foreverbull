import logging
import multiprocessing
import os

from multiprocessing.queues import Queue
from threading import Thread

import pandas as pd
import pytest

from zipline.data import bundles

from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.backtest import ingestion_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_zipline.engine import ConfigError
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.engine import find_end_timestamp
from foreverbull_zipline.engine import find_start_timestamp
from foreverbull_zipline.service import BacktestService


@pytest.mark.parametrize(
    "start_date,expected_start_date",
    [
        (common_pb2.Date(year=2022, month=1, day=3), pd.Timestamp(year=2022, month=1, day=3)),
        (common_pb2.Date(year=2023, month=1, day=3), pd.Timestamp(year=2023, month=1, day=3)),
        (common_pb2.Date(year=2021, month=1, day=3), pd.Timestamp(year=2022, month=1, day=3)),
        (None, pd.Timestamp(year=2022, month=1, day=3)),
    ],
)
def test_find_start_timestamp(execution: execution_pb2.Execution, start_date, expected_start_date):
    bundle = bundles.load("foreverbull", os.environ, None)

    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=start_date,
            end_date=execution.end_date,
            symbols=execution.symbols,
            benchmark=None,
        )
    )

    start_date = find_start_timestamp(bundle, request)
    assert start_date
    assert start_date == expected_start_date


@pytest.mark.parametrize(
    "end_date,expected_end_date",
    [
        (common_pb2.Date(year=2023, month=12, day=29), pd.Timestamp(year=2023, month=12, day=29)),
        (common_pb2.Date(year=2023, month=6, day=1), pd.Timestamp(year=2023, month=6, day=1)),
        (common_pb2.Date(year=2025, month=1, day=3), pd.Timestamp(year=2023, month=12, day=29)),
        (None, pd.Timestamp(year=2023, month=12, day=29)),
    ],
)
def test_find_end_timestamp(execution: execution_pb2.Execution, end_date, expected_end_date):
    bundle = bundles.load("foreverbull", os.environ, None)

    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=end_date,
            symbols=execution.symbols,
            benchmark=None,
        )
    )

    end_date = find_end_timestamp(bundle, request)
    assert end_date
    assert end_date == expected_end_date


def test_timestamp_no_symbols(execution: execution_pb2.Execution):
    bundle = bundles.load("foreverbull", os.environ, None)

    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=execution.end_date,
            symbols=[],
            benchmark=None,
        )
    )

    with pytest.raises(ConfigError, match="no bundle start_date found"):
        find_start_timestamp(bundle, request)

    with pytest.raises(ConfigError, match="no bundle end_date found"):
        find_end_timestamp(bundle, request)


def test_start_stop(spawn_process):
    execution = Engine()
    execution.start()
    execution.is_ready.wait(3.0)
    execution.stop()
    execution.join(3.0)


def logging_thread(q: Queue):
    while True:
        record = q.get()
        if record is None:
            break
        logger = logging.getLogger(record.name)
        logger.handle(record)


@pytest.fixture(scope="session")
def ensure_ingestion(backtest_entity, fb_database):
    _, verify = fb_database
    verify(backtest_entity)

    service = BacktestService()
    try:
        service.ingestion
    except LookupError:
        ingest_request = engine_service_pb2.IngestRequest(
            ingestion=ingestion_pb2.Ingestion(
                start_date=backtest_entity.start_date,
                end_date=backtest_entity.end_date,
                symbols=backtest_entity.symbols,
            )
        )
        service.Ingest(ingest_request, None)


@pytest.fixture(scope="function")
def engine(ensure_ingestion):
    log_queue = multiprocessing.Queue()
    log_thread = Thread(target=logging_thread, args=(log_queue,))
    log_thread.start()
    execution = Engine(logging_queue=log_queue)
    execution.start()
    execution.is_ready.wait(3.0)
    yield execution
    execution.stop()
    execution.join(3.0)
    log_queue.put_nowait(None)
    log_thread.join(3.0)


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_run_benchmark(execution: execution_pb2.Execution, engine: Engine, benchmark: str | None):
    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=execution.end_date,
            symbols=execution.symbols,
            benchmark=benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest.start_date == execution.start_date
    assert response.backtest.end_date == execution.end_date
    assert response.backtest.symbols == execution.symbols
    if benchmark:
        assert response.backtest.benchmark == benchmark
    else:
        assert response.backtest.benchmark == ""

    while True:
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())


def test_premature_stop(execution: execution_pb2.Execution, engine: Engine):
    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=execution.end_date,
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest

    for _ in range(5):
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())


@pytest.mark.parametrize("symbols", [["AAPL"], ["AAPL", "MSFT"], ["TSLA"]])
def test_multiple_runs_different_symbols(execution: execution_pb2.Execution, engine: Engine, symbols):
    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=execution.end_date,
            symbols=symbols,
            benchmark=None,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest
    assert response.backtest.symbols == symbols

    while True:
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())


def test_run_end_date_none(execution: execution_pb2.Execution, engine: Engine):
    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=None,
            symbols=execution.symbols,
            benchmark=None,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest
    assert response.backtest.end_date == execution.end_date

    while True:
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_get_result(execution: execution_pb2.Execution, engine: Engine, benchmark):
    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=execution.end_date,
            symbols=execution.symbols,
            benchmark=benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest

    while True:
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())

    response = engine.get_backtest_result(engine_service_pb2.GetResultRequest())
    assert len(response.periods)
    if benchmark:
        assert response.periods[-1].benchmark_period_return != 0.0
    else:
        assert response.periods[-1].benchmark_period_return == 0.0


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_broker(execution: execution_pb2.Execution, engine: Engine, benchmark: str | None):
    request = engine_service_pb2.RunBacktestRequest(
        backtest=backtest_pb2.Backtest(
            start_date=execution.start_date,
            end_date=execution.end_date,
            symbols=execution.symbols,
            benchmark=benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio
    engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())

    response = engine.place_orders_and_continue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(symbol=execution.symbols[0], amount=10),
            ]
        )
    )

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions[0].amount == 10

    response = engine.place_orders_and_continue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(symbol=execution.symbols[0], amount=-5),
            ]
        )
    )

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions[0].amount == 5

    response = engine.place_orders_and_continue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(symbol=execution.symbols[0], amount=-5),
            ]
        )
    )

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions == []

    while True:
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())

    response = engine.get_backtest_result(engine_service_pb2.GetResultRequest())
    assert len(response.periods)
    if benchmark:
        assert response.periods[-1].alpha != 0.0
        assert response.periods[-1].beta != 0.0
    else:
        assert response.periods[-1].alpha == 0.0
        assert response.periods[-1].beta == 0.0
