import os
from multiprocessing import Event

import pynng
import pytest

from foreverbull import worker
from foreverbull.entity.backtest import Parameter
from foreverbull.entity.service import Request, Response


@pytest.fixture(scope="function")
def setup_worker(algo_with_parameters, execution):
    survey_address = "ipc:///tmp/worker_pool.ipc"
    survey_socket = pynng.Surveyor0(listen=survey_address)
    survey_socket.recv_timeout = 5000
    survey_socket.sendout = 5000
    state_address = "ipc:///tmp/worker_pool_state.ipc"
    state_socket = pynng.Sub0(listen=state_address)
    state_socket.recv_timeout = 5000
    state_socket.sendout = 5000
    state_socket.subscribe(b"")

    stop_event = Event()

    request_socket = pynng.Req0(listen=f"tcp://127.0.0.1:{execution.port}")
    request_socket.recv_timeout = 5000
    request_socket.send_timeout = 5000

    def setup(worker: worker.Worker):
        w = worker(survey_address, state_address, stop_event, algo_with_parameters)
        w.start()
        msg = state_socket.recv()
        assert msg == b"ready"
        return survey_socket, request_socket

    yield setup

    stop_event.set()
    survey_socket.close()
    state_socket.close()
    request_socket.close()


@pytest.mark.parametrize(
    "parameters,param_error",
    [
        ([Parameter(key="low", default="0", type="int", value="5")], None),
    ],
)
def test_configure_worker(execution, setup_worker, spawn_process, parameters, param_error, process_symbols):
    survey, server_socket = setup_worker(worker.WorkerThread)

    execution.parameters = parameters
    survey.send(Request(task="configure_execution", data=execution).dump())
    response = Response.load(survey.recv())
    assert response.task == "configure_execution"
    assert response.error == param_error
    if param_error:
        return

    survey.send(Request(task="run_execution", data=None).dump())
    response = Response.load(survey.recv())
    assert response.task == "run_execution"
    assert response.error is None

    process_symbols(server_socket, "exc_123")


@pytest.mark.parametrize("workerclass", [worker.WorkerThread, worker.WorkerProcess])
def test_worker(workerclass: worker.Worker, execution, setup_worker, spawn_process, process_symbols):
    if type(workerclass) is worker.WorkerProcess and os.environ.get("THREADED_EXECUTION"):
        pytest.skip("WorkerProcess not supported with THREADED_EXECUTION")

    survey_socket, server_socket = setup_worker(workerclass)

    survey_socket.send(Request(task="configure_execution", data=execution).dump())
    response = Response.load(survey_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None

    survey_socket.send(Request(task="run_execution", data=None).dump())
    response = Response.load(survey_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    process_symbols(server_socket, "exc_123")
