import logging
import multiprocessing
import time
from datetime import datetime, timezone
from multiprocessing import get_start_method, set_start_method
from multiprocessing.queues import Queue

import pynng
import pytest
from foreverbull.entity import backtest
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2
from foreverbull.pb.service import service_pb2
from foreverbull_zipline.execution import Execution, ExecutionProcess


def test_start_stop():
    execution = ExecutionProcess()
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
        print("RECORD: ", record)


from threading import Thread


@pytest.fixture
def execution_process():
    log_queue = multiprocessing.Queue()
    log_thread = Thread(target=logging_thread, args=(log_queue,))
    log_thread.start()
    execution = ExecutionProcess(logging_queue=log_queue)
    execution.start()
    execution.is_ready.wait(3.0)
    yield execution
    execution.stop()
    execution.join(3.0)
    log_queue.put_nowait(None)
    log_thread.join(3.0)


def test_ingest(
    fb_database,
    execution_process: Execution,
    backtest_entity: backtest.Backtest,
):
    _, ingest = fb_database
    ingest(backtest_entity)
    ingest_request = backtest_pb2.IngestRequest(
        ingestion=backtest_pb2.Ingestion(
            start_date=pb_utils.to_proto_timestamp(backtest_entity.start),
            end_date=pb_utils.to_proto_timestamp(backtest_entity.end),
            symbols=backtest_entity.symbols,
        )
    )
    response = execution_process.ingest(ingest_request)
    assert response.ingestion.start_date == pb_utils.to_proto_timestamp(backtest_entity.start)
    assert response.ingestion.end_date == pb_utils.to_proto_timestamp(backtest_entity.end)
    assert response.ingestion.symbols == backtest_entity.symbols


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_run_benchmark(execution: backtest.Execution, execution_process: Execution, benchmark: str | None):
    request = backtest_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=benchmark,
        )
    )
    response = execution_process.run_backtest(request)

    while True:
        response = execution_process.continue_backtest(backtest_pb2.ContinueRequest())
        if response.is_running is False:
            break


def test_premature_stop(execution: backtest.Execution, execution_process: Execution):
    request = backtest_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
    )
    response = execution_process.run_backtest(request)

    for _ in range(10):
        response = execution_process.continue_backtest(backtest_pb2.ContinueRequest())

    execution_process.stop()


@pytest.mark.parametrize("symbols", [["AAPL"], ["AAPL", "MSFT"], ["TSLA"]])
def test_multiple_runs_different_symbols(execution: backtest.Execution, execution_process: Execution, symbols):
    request = backtest_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=symbols,
        )
    )
    response = execution_process.run_backtest(request)

    while True:
        response = execution_process.continue_backtest(backtest_pb2.ContinueRequest())
        if response.is_running is False:
            break


def test_get_result(execution: backtest.Execution, execution_process: Execution):
    request = backtest_pb2.RunRequest(
        backtest=backtest_pb2.Backtest(
            start_date=pb_utils.to_proto_timestamp(execution.start),
            end_date=pb_utils.to_proto_timestamp(execution.end),
            symbols=execution.symbols,
            benchmark=execution.benchmark,
        )
    )
    response = execution_process.run_backtest(request)

    while True:
        response = execution_process.continue_backtest(backtest_pb2.ContinueRequest())
        if response.is_running is False:
            break

    response = execution_process.get_backtest_result(backtest_pb2.GetResultRequest())
    assert len(response.periods)


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_broker(execution: backtest.Execution, execution_process: Execution, benchmark: str | None):
    ce_request = backtest_pb2.ConfigureRequest(
        start_date=pb_utils.to_proto_timestamp(execution.start),
        end_date=pb_utils.to_proto_timestamp(execution.end),
        symbols=execution.symbols,
        benchmark=benchmark,
    )
    request = service_pb2.Request(task="configure_execution", data=ce_request.SerializeToString())
    execution_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.HasField("error") is False

    execution_socket.send(service_pb2.Request(task="run_execution").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.HasField("error") is False

    execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "continue"
    assert response.HasField("error") is False

    continue_request = backtest_pb2.ContinueRequest(
        orders=[
            backtest_pb2.Order(symbol=execution.symbols[0], amount=10),
        ]
    )
    request = service_pb2.Request(task="continue", data=continue_request.SerializeToString())
    execution_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "continue"
    assert response.HasField("error") is False

    execution_socket.send(service_pb2.Request(task="get_portfolio").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "get_portfolio"
    assert response.HasField("error") is False
    portfolio = backtest_pb2.GetPortfolioResponse()
    portfolio.ParseFromString(response.data)
    assert len(portfolio.positions) == 1
    assert portfolio.positions[0].symbol == execution.symbols[0]

    execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "continue"
    assert response.HasField("error") is False

    continue_request = backtest_pb2.ContinueRequest(
        orders=[
            backtest_pb2.Order(symbol=execution.symbols[0], amount=15),
        ]
    )
    request = service_pb2.Request(task="continue", data=continue_request.SerializeToString())
    execution_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "continue"
    assert response.HasField("error") is False

    while True:
        execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "continue"
        if response.HasField("error"):
            assert response.error == "no active execution"
            break

    execution_socket.send(service_pb2.Request(task="get_execution_result").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "get_execution_result"
    assert response.HasField("error") is False
    result = backtest_pb2.ResultResponse()
    result.ParseFromString(response.data)
    assert len(result.periods)
