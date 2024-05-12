import os
from datetime import timedelta
from threading import Thread

import pynng
import pytest

from foreverbull import socket, worker
from foreverbull.entity.finance import Portfolio
from foreverbull.socket import Request, Response


@pytest.fixture(scope="session")
def process_symbols(ingest_config):
    def _process_symbols(server_socket, parallel):
        start = ingest_config.start
        portfolio = Portfolio(cash=0, value=0, positions=[])
        while start < ingest_config.end:
            if parallel:
                for symbol in ingest_config.symbols:
                    req = worker.Request(timestamp=start, symbols=[symbol], portfolio=portfolio)
                    server_socket.send(Request(task="", data=req).serialize())
                    response = Response.deserialize(server_socket.recv())
                    assert response.task == ""
                    assert response.error is None
            else:
                req = worker.Request(timestamp=start, symbols=ingest_config.symbols, portfolio=portfolio)
                server_socket.send(Request(task="", data=req).serialize())
                response = Response.deserialize(server_socket.recv())
                assert response.task == ""
                assert response.error is None
            start += timedelta(days=1)

    return _process_symbols


@pytest.fixture
def namespace_server():
    namespace = dict()

    s = pynng.Rep0(listen="tcp://0.0.0.0:7878")
    s.recv_timeout = 500
    s.send_timeout = 500
    os.environ["NAMESPACE_PORT"] = "7878"

    def runner(s, namespace):
        while True:
            try:
                message = s.recv()
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            request = socket.Request.deserialize(message)
            if request.task.startswith("get:"):
                key = request.task[4:]
                response = socket.Response(task=request.task, data=namespace.get(key))
                s.send(response.serialize())
            elif request.task.startswith("set:"):
                key = request.task[4:]
                namespace[key] = request.data
                response = socket.Response(task=request.task)
                s.send(response.serialize())
            else:
                response = socket.Response(task=request.task, error="Invalid task")
                s.send(response.serialize())

    thread = Thread(target=runner, args=(s, namespace))
    thread.start()

    yield namespace

    s.close()
    thread.join()
