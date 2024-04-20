import os
import time

import pynng
import pytest

from foreverbull import Foreverbull, entity, worker
from foreverbull_zipline.entity import Period
from foreverbull_zipline.execution import Execution


@pytest.fixture
def zipline_socket():
    execution = Execution(port=6666)
    execution.start()
    for _ in range(10):
        try:
            socket = pynng.Req0(
                dial=f"tcp://{execution.socket_config.host}:{execution.socket_config.port}",
                block_on_dial=True,
            )
            socket.recv_timeout = 10000
            socket.sendout = 10000
            break
        except pynng.exceptions.ConnectionRefused:
            time.sleep(0.1)
    else:
        raise Exception("Failed to connect to execution socket")

    def run(
        execution: entity.backtest.Execution,
    ):
        socket.send(
            entity.service.Request(
                task="configure_execution",
                data=execution,
            ).dump()
        )
        response = entity.service.Response.load(socket.recv())
        if response.error:
            raise Exception(response.error)
        socket.send(entity.service.Request(task="run_execution").dump())
        response = entity.service.Response.load(socket.recv())
        if response.error:
            raise Exception(response.error)
        return socket

    yield run
    socket.close()
    execution.stop()
    execution.join()


@pytest.mark.parametrize(
    "file_path,configuration",
    [
        (
            "example_parallel.py",
            {"handle_data": entity.service.Execution.Function(parameters={})},
        ),
        (
            "example_sequential.py",
            {"handle_data": entity.service.Execution.Function(parameters={})},
        ),
    ],
)
def test_integration(
    zipline_socket,
    execution,
    database,
    ingest_config,
    file_path,
    configuration,
):
    execution_socket = pynng.Req0(listen="tcp://0.0.0.0:8888")
    execution_socket.recv_timeout = 10000
    execution_socket.sendout = 10000

    session = entity.backtest.Session(
        backtest="test",
        manual=False,
        executions=0,
    )

    service_execution = entity.service.Execution(
        id="test",
        port=8888,
        database_url=os.environ.get(
            "DATABASE_URL",
            "",
        ),
        configuration=configuration,
    )

    with Foreverbull(
        session,
        file_path,
    ):
        backtest = zipline_socket(execution)
        service_socket = pynng.Req0(dial="tcp://127.0.0.1:5555")
        service_socket.recv_timeout = 10000
        service_socket.sendout = 10000

        service_socket.send(entity.service.Request(task="info").dump())
        response = entity.service.Response.load(service_socket.recv())
        assert response.error is None
        service = entity.service.Service(**response.data)

        is_parallel = service.algorithm.functions[0].parallel_execution

        service_socket.send(
            entity.service.Request(
                task="configure_execution",
                data=service_execution,
            ).dump()
        )
        response = entity.service.Response.load(service_socket.recv())
        assert response.error is None

        service_socket.send(entity.service.Request(task="run_execution").dump())
        response = entity.service.Response.load(service_socket.recv())
        assert response.error is None

        while True:
            backtest.send(entity.service.Request(task="get_period").dump())
            try:
                period = Period(**entity.service.Response.load(backtest.recv()).data)
            except TypeError:
                break
            portfolio = entity.finance.Portfolio(
                cash=period.cash,
                value=period.positions_value,
                positions=[
                    entity.finance.Position(
                        symbol=position.symbol,
                        amount=position.amount,
                        cost_basis=position.cost_basis,
                    )
                    for position in period.positions
                ],
            )
            if is_parallel:
                for symbol in ingest_config.symbols:
                    req = worker.Request(
                        execution=service_execution,
                        timestamp=period.timestamp,
                        symbol=symbol,
                        portfolio=portfolio,
                    )
                    execution_socket.send(
                        entity.service.Request(
                            task="handle_data",
                            data=req,
                        ).dump()
                    )
                    response = entity.service.Response.load(execution_socket.recv())
                    assert response.error is None
                    if response.data:
                        order = entity.finance.Order(**response.data)
                        backtest.send(
                            entity.service.Request(
                                task="order",
                                data=order,
                            ).dump()
                        )
                        response = entity.service.Response.load(backtest.recv())
                        assert response.error is None
            else:
                req = worker.Request(
                    execution=service_execution,
                    timestamp=period.timestamp,
                    symbols=ingest_config.symbols,
                    portfolio=portfolio,
                )
                execution_socket.send(
                    entity.service.Request(
                        task="handle_data",
                        data=req,
                    ).dump()
                )
                response = entity.service.Response.load(execution_socket.recv())
                assert response.error is None
                if response.data:
                    for order in response.data:
                        o = entity.finance.Order(**order)
                        backtest.send(
                            entity.service.Request(
                                task="order",
                                data=o,
                            ).dump()
                        )
                        response = entity.service.Response.load(backtest.recv())
                        assert response.error is None

            backtest.send(entity.service.Request(task="continue").dump())
            response = entity.service.Response.load(backtest.recv())
            assert response.error is None

        service_socket.close()
        execution_socket.close()
