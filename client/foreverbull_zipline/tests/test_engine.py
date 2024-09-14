import logging
import multiprocessing
from multiprocessing.queues import Queue
from threading import Thread

import pytest
from foreverbull.entity import backtest
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, engine_pb2
from foreverbull.pb.finance import finance_pb2
from foreverbull_zipline.engine import Engine, EngineProcess


def test_start_stop():
    execution = EngineProcess()
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


@pytest.fixture(scope="function")
def engine():
    log_queue = multiprocessing.Queue()
    log_thread = Thread(target=logging_thread, args=(log_queue,))
    log_thread.start()
    execution = EngineProcess(logging_queue=log_queue)
    execution.start()
    execution.is_ready.wait(3.0)
    yield execution
    execution.stop()
    execution.join(3.0)
    log_queue.put_nowait(None)
    log_thread.join(3.0)


def test_ingest(
    fb_database,
    engine: Engine,
    backtest_entity: backtest.Backtest,
):
    _, ingest = fb_database
    ingest(backtest_entity)
    ingest_request = engine_pb2.IngestRequest(
        ingestion=backtest_pb2.Ingestion(
            start_date=pb_utils.to_proto_timestamp(backtest_entity.start),
            end_date=pb_utils.to_proto_timestamp(backtest_entity.end),
            symbols=backtest_entity.symbols,
        )
    )
    response = engine.ingest(ingest_request)
    assert response.ingestion.start_date == pb_utils.to_proto_timestamp(backtest_entity.start)
    assert response.ingestion.end_date == pb_utils.to_proto_timestamp(backtest_entity.end)
    assert response.ingestion.symbols == backtest_entity.symbols


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_run_benchmark(execution: backtest.Execution, engine: Engine, benchmark: str | None):
    request = engine_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest.start_date == pb_utils.to_proto_timestamp(execution.start)
    assert response.backtest.end_date == pb_utils.to_proto_timestamp(execution.end)
    assert response.backtest.symbols == execution.symbols
    if benchmark:
        assert response.backtest.benchmark == benchmark
    else:
        assert response.backtest.benchmark == ""

    while True:
        response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_pb2.PlaceOrdersAndContinueRequest())


def test_premature_stop(execution: backtest.Execution, engine: Engine):
    request = engine_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest

    for _ in range(5):
        response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_pb2.PlaceOrdersAndContinueRequest())


@pytest.mark.parametrize("symbols", [["AAPL"], ["AAPL", "MSFT"], ["TSLA"]])
def test_multiple_runs_different_symbols(execution: backtest.Execution, engine: Engine, symbols):
    request = engine_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=symbols,
            benchmark=None,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest
    assert response.backtest.symbols == symbols

    while True:
        response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_pb2.PlaceOrdersAndContinueRequest())


def test_get_result(execution: backtest.Execution, engine: Engine):
    request = engine_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest

    while True:
        response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_pb2.PlaceOrdersAndContinueRequest())

    response = engine.get_backtest_result(engine_pb2.GetResultRequest())
    assert len(response.periods)


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_broker(execution: backtest.Execution, engine: Engine, benchmark: str | None):
    request = engine_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
    )
    response = engine.run_backtest(request)
    assert response.backtest

    response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio
    engine.place_orders_and_continue(engine_pb2.PlaceOrdersAndContinueRequest())

    response = engine.place_orders_and_continue(
        engine_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(symbol=execution.symbols[0], amount=10),
            ]
        )
    )

    response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions[0].amount == 10

    response = engine.place_orders_and_continue(
        engine_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(symbol=execution.symbols[0], amount=-5),
            ]
        )
    )

    response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions[0].amount == 5

    response = engine.place_orders_and_continue(
        engine_pb2.PlaceOrdersAndContinueRequest(
            orders=[
                finance_pb2.Order(symbol=execution.symbols[0], amount=-5),
            ]
        )
    )

    response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
    assert response.is_running
    assert response.portfolio.positions == []

    while True:
        response = engine.get_current_period(engine_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        engine.place_orders_and_continue(engine_pb2.PlaceOrdersAndContinueRequest())

    response = engine.get_backtest_result(engine_pb2.GetResultRequest())
    assert len(response.periods)
