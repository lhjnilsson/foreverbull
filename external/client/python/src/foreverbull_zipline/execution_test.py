import logging
import time
from datetime import datetime, timezone

import pynng
import pytest

from foreverbull.entity import backtest
from foreverbull.socket import Request, Response
from foreverbull_zipline import entity
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
            socket.sendout = 10000
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
    req = Request(task="info")
    execution_socket.send(req.serialize())
    rsp_data = execution_socket.recv()
    rsp = Response.deserialize(rsp_data)
    assert rsp.task == "info"
    assert rsp.error is None
    assert rsp.data["type"] == "backtest"
    assert "socket" in rsp.data
    assert "host" in rsp.data["socket"]
    assert "port" in rsp.data["socket"]


def test_ingest(
    database,
    execution_socket: pynng.Rep0,
    ingest_config,
):
    execution_socket.send(Request(task="ingest", data=ingest_config).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "ingest"
    assert response.error is None


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_run_benchmark(execution: Execution, execution_socket: pynng.Rep0, benchmark):
    execution_socket.send(Request(task="info").serialize())
    Response.deserialize(execution_socket.recv())

    execution.benchmark = benchmark

    execution_socket.send(Request(task="configure_execution", data=execution).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None

    execution_socket.send(Request(task="run_execution").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    while True:
        execution_socket.send(Request(task="get_period").serialize())
        response = Response.deserialize(execution_socket.recv())
        assert response.task == "get_period"
        if response.data is None:
            break
        assert response.error is None
        execution_socket.send(Request(task="continue").serialize())
        execution_socket.recv()


# None value should use latest or greatest dates
# 2022-12-01 is before ingested start date, should automatically be set to earliest ingested date
# 2023-04-30 is after ingested end date, should automatically be set to latest ingested date
@pytest.mark.parametrize(
    "start",
    [
        None,
        datetime(2023, 1, 3, tzinfo=timezone.utc),
        datetime(2023, 2, 1, tzinfo=timezone.utc),
        datetime(2022, 12, 1, tzinfo=timezone.utc),
    ],
)
@pytest.mark.parametrize(
    "end",
    [
        None,
        datetime(2023, 3, 30, tzinfo=timezone.utc),
        datetime(2023, 2, 1, tzinfo=timezone.utc),
        datetime(2023, 4, 30, tzinfo=timezone.utc),
    ],
)
def test_run_with_time(execution: Execution, execution_socket: pynng.Rep0, ingest_config, start, end):
    execution_socket.send(Request(task="info").serialize())
    Response.deserialize(execution_socket.recv())

    execution.start = start
    execution.end = end
    execution_socket.send(Request(task="configure_execution", data=execution).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None
    execution = backtest.Execution(**response.data)

    execution_socket.send(Request(task="run_execution").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    if start is None:
        assert execution.start == ingest_config.start
    elif start < ingest_config.start:
        assert execution.start == ingest_config.start
    elif start > ingest_config.start:
        assert execution.start == start

    if end is None:
        assert execution.end == ingest_config.end
    elif end > ingest_config.end:
        assert execution.end == ingest_config.end
    elif end < ingest_config.end:
        assert execution.end == end

    while True:
        execution_socket.send(Request(task="get_period").serialize())
        response = Response.deserialize(execution_socket.recv())
        assert response.task == "get_period"
        if response.data is None:
            break
        assert response.error is None
        execution_socket.send(Request(task="continue").serialize())
        execution_socket.recv()


def test_premature_stop(execution: Execution, execution_socket: pynng.Rep0):
    execution_socket.send(Request(task="info").serialize())
    Response.deserialize(execution_socket.recv())

    execution_socket.send(Request(task="configure_execution", data=execution).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None

    execution_socket.send(Request(task="run_execution").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    for _ in range(10):
        execution_socket.send(Request(task="get_period").serialize())
        response = Response.deserialize(execution_socket.recv())
        assert response.task == "get_period"
        assert response.error is None
        execution_socket.send(Request(task="continue").serialize())
        execution_socket.recv()

    execution_socket.send(Request(task="stop").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "stop"
    assert response.error is None


# None should be all ingested symbols
@pytest.mark.parametrize("symbols", [["AAPL"], ["AAPL", "MSFT"], ["TSLA"], None])
def test_multiple_runs_different_symbols(execution: Execution, execution_socket: pynng.Rep0, symbols):
    execution.symbols = symbols
    execution_socket.send(Request(task="configure_execution", data=execution).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None
    execution = backtest.Execution(**response.data)
    assert execution.symbols == symbols if symbols is not None else ["AAPL", "MSFT", "TSLA"]

    execution_socket.send(Request(task="run_execution").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    while True:
        execution_socket.send(Request(task="get_period").serialize())
        response = Response.deserialize(execution_socket.recv())
        assert response.task == "get_period"
        if response.data is None:
            break
        assert response.error is None
        execution_socket.send(Request(task="continue").serialize())
        execution_socket.recv()


def test_get_result(execution: Execution, execution_socket: pynng.Rep0):
    execution_socket.send(Request(task="configure_execution", data=execution).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None

    execution_socket.send(Request(task="run_execution").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    while True:
        execution_socket.send(Request(task="get_period").serialize())
        response = Response.deserialize(execution_socket.recv())
        assert response.task == "get_period"
        if response.data is None:
            break
        assert response.error is None
        execution_socket.send(Request(task="continue").serialize())
        execution_socket.recv()

    execution_socket.send(Request(task="get_execution_result").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_execution_result"
    assert response.error is None
    assert "periods" in response.data
    assert len(response.data["periods"])


@pytest.mark.parametrize("benchmark", ["AAPL", None])
def test_broker(execution: Execution, execution_socket: pynng.Rep0, benchmark):
    execution.benchmark = benchmark

    execution_socket.send(Request(task="configure_execution", data=execution).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None

    execution_socket.send(Request(task="run_execution").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    execution_socket.send(Request(task="continue").serialize())
    execution_socket.recv()

    asset = entity.Asset(symbol=execution.symbols[0])
    execution_socket.send(Request(task="can_trade", data=asset).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "can_trade"
    assert response.error is None

    order = entity.Order(symbol=execution.symbols[0], amount=10)
    execution_socket.send(Request(task="order", data=order).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "order"
    assert response.error is None
    assert response.data["symbol"] == order.symbol
    assert response.data["amount"] == order.amount
    assert response.data["status"] == entity.OrderStatus.OPEN
    assert response.data["id"]
    placed_order = response.data

    execution_socket.send(Request(task="continue").serialize())
    execution_socket.recv()

    execution_socket.send(Request(task="get_period").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_period"
    assert response.error is None
    period = entity.Period(**response.data)
    assert len(period.new_orders) == 1
    assert period.new_orders[0].symbol == order.symbol

    execution_socket.send(Request(task="get_order", data=placed_order).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_order"
    assert response.error is None
    assert response.data["symbol"] == order.symbol
    assert response.data["amount"] == order.amount
    assert response.data["status"] == entity.OrderStatus.FILLED

    execution_socket.send(Request(task="continue").serialize())
    execution_socket.recv()

    execution_socket.send(Request(task="get_period").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_period"
    assert response.error is None
    period = entity.Period(**response.data)
    assert len(period.new_orders) == 0

    order = entity.Order(symbol=execution.symbols[0], amount=15)
    execution_socket.send(Request(task="order", data=order).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "order"
    assert response.error is None
    assert response.data["symbol"] == order.symbol
    assert response.data["amount"] == order.amount
    assert response.data["status"] == entity.OrderStatus.OPEN
    assert response.data["id"]

    execution_socket.send(Request(task="get_open_orders").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_open_orders"
    assert response.error is None
    assert "orders" in response.data
    assert len(response.data["orders"]) == 1
    assert response.data["orders"][0]["symbol"] == order.symbol
    assert response.data["orders"][0]["amount"] == order.amount
    assert response.data["orders"][0]["status"] == entity.OrderStatus.OPEN

    execution_socket.send(Request(task="cancel_order", data=response.data["orders"][0]).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "cancel_order"
    assert response.error is None

    execution_socket.send(Request(task="get_order", data=response.data).serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_order"
    assert response.error is None
    assert response.data["symbol"] == order.symbol
    assert response.data["amount"] == order.amount
    assert response.data["status"] == entity.OrderStatus.CANCELLED

    while True:
        execution_socket.send(Request(task="continue").serialize())
        response = Response.deserialize(execution_socket.recv())
        if response.error and response.error == "no active execution":
            break

    execution_socket.send(Request(task="get_execution_result").serialize())
    response = Response.deserialize(execution_socket.recv())
    assert response.task == "get_execution_result"
    assert response.error is None
    assert "periods" in response.data
    assert len(response.data["periods"])

    print(response.data["periods"][-1]["benchmark_period_return"])
