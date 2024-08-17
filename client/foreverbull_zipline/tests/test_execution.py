import logging
import time
from datetime import datetime, timezone

import pynng
import pytest
from foreverbull.entity import backtest
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import engine_pb2
from foreverbull.pb.service import service_pb2
from foreverbull_zipline.execution import Execution


def test_start_stop():
    execution = Execution()
    execution.start()
    time.sleep(0.5)  # Make sure it has time to start
    execution.stop()


@pytest.fixture
def execution_socket():
    execution = Execution()
    execution.start()

    # retry creation of socket, in case previous tests have not closed properly
    for _ in range(10):
        try:
            socket = pynng.Req0(
                dial=f"tcp://{execution.socket_config.host}:{execution.socket_config.port}", block_on_dial=True
            )
            socket.recv_timeout = 10000
            socket.send_timeout = 10000
            break
        except pynng.exceptions.ConnectionRefused:
            logging.getLogger("execution-test").warning("Failed to connect to execution socket, retrying...")
            time.sleep(0.1)
    else:
        raise Exception("Failed to connect to execution socket")

    yield socket

    execution.stop()
    socket.close()


def test_info(execution_socket: pynng.Rep0):
    req = service_pb2.Request(task="info")
    execution_socket.send(req.SerializeToString())
    rsp_data = execution_socket.recv()
    rsp = service_pb2.Response()
    rsp.ParseFromString(rsp_data)
    assert rsp.task == "info"
    assert rsp.HasField("error") is False

    service_info = service_pb2.ServiceInfoResponse()
    service_info.ParseFromString(rsp.data)
    assert service_info.serviceType == "backtest"


def test_ingest(
    fb_database,
    execution_socket: pynng.Rep0,
    backtest_entity: backtest.Backtest,
):
    _, ingest = fb_database
    ingest(backtest_entity)
    ingest_request = engine_pb2.IngestRequest(
        start_date=pb_utils.to_proto_timestamp(backtest_entity.start),
        end_date=pb_utils.to_proto_timestamp(backtest_entity.end),
        symbols=backtest_entity.symbols,
    )
    request = service_pb2.Request(task="ingest", data=ingest_request.SerializeToString())
    execution_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "ingest"
    assert response.HasField("error") is False
    ingest_response = engine_pb2.IngestResponse()
    ingest_response.ParseFromString(response.data)
    assert ingest_response.start_date == pb_utils.to_proto_timestamp(backtest_entity.start)
    assert ingest_response.end_date == pb_utils.to_proto_timestamp(backtest_entity.end)
    assert ingest_response.symbols == backtest_entity.symbols


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_run_benchmark(execution: backtest.Execution, execution_socket: pynng.Rep0, benchmark: str | None):
    ce_request = engine_pb2.ConfigureRequest(
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

    while True:
        execution_socket.send(service_pb2.Request(task="get_portfolio").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "get_portfolio"
        if response.HasField("data") is False:
            break
        assert response.HasField("error") is False
        portfolio = engine_pb2.GetPortfolioResponse()
        portfolio.ParseFromString(response.data)

        continue_request = engine_pb2.ContinueRequest()
        execution_socket.send(
            service_pb2.Request(task="continue", data=continue_request.SerializeToString()).SerializeToString()
        )
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "continue"
        assert response.HasField("error") is False


# None value should use latest or greatest dates
# 2022-12-01 is before ingested start date, should automatically be set to earliest ingested date
# 2023-04-30 is after ingested end date, should automatically be set to latest ingested date
@pytest.mark.parametrize(
    "start",
    [
        datetime(2023, 1, 3, tzinfo=timezone.utc),
        datetime(2023, 2, 1, tzinfo=timezone.utc),
        datetime(2022, 12, 1, tzinfo=timezone.utc),
    ],
)
@pytest.mark.parametrize(
    "end",
    [
        datetime(2023, 3, 30, tzinfo=timezone.utc),
        datetime(2023, 2, 1, tzinfo=timezone.utc),
        datetime(2023, 4, 30, tzinfo=timezone.utc),
    ],
)
@pytest.mark.skip(reason="unsure how to handle this, if we should raise exception is date is after possible date")
def test_run_with_time(execution: backtest.Execution, execution_socket: pynng.Rep0, backtest_entity, start, end):
    ce_request = engine_pb2.ConfigureRequest(
        start_date=pb_utils.to_proto_timestamp(start),
        end_date=pb_utils.to_proto_timestamp(end),
        symbols=execution.symbols,
        benchmark=execution.benchmark,
    )
    request = service_pb2.Request(task="configure_execution", data=ce_request.SerializeToString())
    execution_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.HasField("error") is False

    ce_response = engine_pb2.ConfigureResponse()
    ce_response.ParseFromString(response.data)
    assert ce_response.start_date == pb_utils.to_proto_timestamp(start)
    assert ce_response.end_date == pb_utils.to_proto_timestamp(end)
    assert ce_response.symbols == execution.symbols
    assert ce_response.benchmark == execution.benchmark

    execution_socket.send(service_pb2.Request(task="run_execution").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.HasField("error") is False

    while True:
        execution_socket.send(service_pb2.Request(task="get_portfolio").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "get_portfolio"
        if response.HasField("data") is False:
            break
        assert response.HasField("error") is False
        portfolio = engine_pb2.GetPortfolioResponse()
        portfolio.ParseFromString(response.data)

        continue_request = engine_pb2.ContinueRequest()
        execution_socket.send(
            service_pb2.Request(task="continue", data=continue_request.SerializeToString()).SerializeToString()
        )
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "continue"
        assert response.HasField("error") is False


def test_premature_stop(execution: backtest.Execution, execution_socket: pynng.Rep0):
    ce_request = engine_pb2.ConfigureRequest(
        start_date=pb_utils.to_proto_timestamp(execution.start),
        end_date=pb_utils.to_proto_timestamp(execution.end),
        symbols=execution.symbols,
        benchmark=execution.benchmark,
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

    for _ in range(10):
        execution_socket.send(service_pb2.Request(task="get_portfolio").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "get_portfolio"
        assert response.HasField("error") is False

        execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "continue"
        assert response.HasField("error") is False

    execution_socket.send(service_pb2.Request(task="stop").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "stop"
    assert response.HasField("error") is False


@pytest.mark.parametrize("symbols", [["AAPL"], ["AAPL", "MSFT"], ["TSLA"]])
def test_multiple_runs_different_symbols(execution: backtest.Execution, execution_socket: pynng.Rep0, symbols):
    ce_request = engine_pb2.ConfigureRequest(
        start_date=pb_utils.to_proto_timestamp(execution.start),
        end_date=pb_utils.to_proto_timestamp(execution.end),
        symbols=symbols,
        benchmark=execution.benchmark,
    )
    request = service_pb2.Request(task="configure_execution", data=ce_request.SerializeToString())
    execution_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.HasField("error") is False
    ce_response = engine_pb2.ConfigureResponse()
    ce_response.ParseFromString(response.data)
    assert ce_response.start_date == pb_utils.to_proto_timestamp(execution.start)
    assert ce_response.end_date == pb_utils.to_proto_timestamp(execution.end)
    assert ce_response.symbols == symbols
    assert ce_response.benchmark == execution.benchmark

    execution_socket.send(service_pb2.Request(task="run_execution").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.HasField("error") is False

    while True:
        execution_socket.send(service_pb2.Request(task="get_portfolio").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "get_portfolio"
        if response.HasField("data") is False:
            break
        assert response.HasField("error") is False
        execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "continue"
        assert response.HasField("error") is False


def test_get_result(execution: backtest.Execution, execution_socket: pynng.Rep0):
    ce_request = engine_pb2.ConfigureRequest(
        start_date=pb_utils.to_proto_timestamp(execution.start),
        end_date=pb_utils.to_proto_timestamp(execution.end),
        symbols=execution.symbols,
        benchmark=execution.benchmark,
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

    while True:
        execution_socket.send(service_pb2.Request(task="get_portfolio").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "get_portfolio"
        if response.HasField("data") is False:
            break
        assert response.HasField("error") is False
        execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
        response = service_pb2.Response()
        response.ParseFromString(execution_socket.recv())
        assert response.task == "continue"
        assert response.HasField("error") is False

    execution_socket.send(service_pb2.Request(task="get_execution_result").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "get_execution_result"
    assert response.HasField("error") is False
    result = engine_pb2.ResultResponse()
    result.ParseFromString(response.data)
    assert len(result.periods)


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_broker(execution: backtest.Execution, execution_socket: pynng.Rep0, benchmark: str | None):
    ce_request = engine_pb2.ConfigureRequest(
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

    continue_request = engine_pb2.ContinueRequest(
        orders=[
            engine_pb2.Order(symbol=execution.symbols[0], amount=10),
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
    portfolio = engine_pb2.GetPortfolioResponse()
    portfolio.ParseFromString(response.data)
    assert len(portfolio.positions) == 1
    assert portfolio.positions[0].symbol == execution.symbols[0]

    execution_socket.send(service_pb2.Request(task="continue").SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(execution_socket.recv())
    assert response.task == "continue"
    assert response.HasField("error") is False

    continue_request = engine_pb2.ContinueRequest(
        orders=[
            engine_pb2.Order(symbol=execution.symbols[0], amount=15),
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
    result = engine_pb2.ResultResponse()
    result.ParseFromString(response.data)
    assert len(result.periods)
