import logging
import multiprocessing
import os

from multiprocessing.queues import Queue
from threading import Thread

import pytest

from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.service import BacktestService

from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2 as engine_service_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
from zipline.data import bundles


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
    bundles.register(backtest_entity.name, SQLIngester(backtest_entity), calendar_name="XNYS")
    try:
        bundles.load(backtest_entity.name, os.environ, None)
    except ValueError:
        service = BacktestService()
        service.Ingest(engine_service_pb2.IngestRequest(backtest=backtest_entity), None)


@pytest.fixture(scope="function")
def engine(ensure_ingestion):
    log_queue = multiprocessing.Queue()
    log_thread = Thread(target=logging_thread, args=(log_queue,))
    log_thread.start()
    execution = Engine(logging_queue=log_queue)
    try:
        execution.start()
        execution.is_ready.wait(3.0)
        yield execution
    finally:
        execution.stop()
        execution.join(3.0)
        log_queue.put_nowait(None)
        log_thread.join(3.0)


def test_premature_stop(execution: execution_pb2.Execution, engine: Engine):
    request = engine_service_pb2.RunBacktestRequest(
        execution=execution,
    )
    response = engine.run_backtest(request)
    assert response.execution

    for _ in range(5):
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())


def test_get_result(
    execution: execution_pb2.Execution,
    backtest_entity: backtest_pb2.Backtest,
    engine: Engine,
):
    request = engine_service_pb2.RunBacktestRequest(
        execution=execution,
        benchmark=backtest_entity.benchmark,
    )
    response = engine.run_backtest(request)
    assert response.execution

    while True:
        response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())

    response = engine.get_backtest_result(engine_service_pb2.GetResultRequest())
    assert len(response.periods)
    assert response.periods[-1].benchmark_period_return != 0.0


def test_broker(
    backtest_entity: backtest_pb2.Backtest,
    execution: execution_pb2.Execution,
    engine: Engine,
):
    request = engine_service_pb2.RunBacktestRequest(
        execution=execution,
        benchmark=backtest_entity.benchmark,
    )
    response = engine.run_backtest(request)
    assert response.execution

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio
    engine.place_orders_and_continue(engine_service_pb2.PlaceOrdersAndContinueRequest())

    response = engine.place_orders_and_continue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                marketdata_pb2.Order(symbol=backtest_entity.periods[0].assets[0].symbol, amount=10),
            ]
        )
    )

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions[0].amount == 10

    response = engine.place_orders_and_continue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                marketdata_pb2.Order(symbol=backtest_entity.periods[0].assets[0].symbol, amount=-5),
            ]
        )
    )

    response = engine.get_current_period(engine_service_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions[0].amount == 5

    response = engine.place_orders_and_continue(
        engine_service_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                marketdata_pb2.Order(symbol=backtest_entity.periods[0].assets[0].symbol, amount=-5),
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
    assert response.periods[-1].alpha != 0.0
    assert response.periods[-1].beta != 0.0
