import logging
import multiprocessing

from multiprocessing.queues import Queue
from threading import Thread

import pytest

from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.backtest import ingestion_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.service import BacktestService


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
def ensure_ingestion(backtest_entity):
    service = BacktestService()
    try:
        service.ingestion
    except ValueError:
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

    engine.stop()


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
