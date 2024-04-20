import os
from multiprocessing import Event

import pynng
import pytest

from foreverbull import worker
from foreverbull.entity.service import Request, Response


@pytest.fixture(scope="function")
def setup_worker():
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

    request_socket = pynng.Req0(listen="tcp://127.0.0.1:5656")
    request_socket.recv_timeout = 5000
    request_socket.send_timeout = 5000

    def setup(
        worker: worker.Worker,
        file_name,
        execution,
    ):
        w = worker(
            survey_address,
            state_address,
            stop_event,
            file_name,
        )
        w.start()
        msg = state_socket.recv()
        assert msg == b"ready"
        return (
            survey_socket,
            request_socket,
        )

    yield setup

    stop_event.set()
    survey_socket.close()
    state_socket.close()
    request_socket.close()


@pytest.mark.parametrize(
    "workerclass",
    [
        worker.WorkerThread,
        worker.WorkerProcess,
    ],
)
def test_worker(
    workerclass: worker.Worker,
    setup_worker,
    parallel_algo,
    spawn_process,
):
    if type(workerclass) is worker.WorkerProcess and os.environ.get("THREADED_EXECUTION"):
        pytest.skip("WorkerProcess not supported with THREADED_EXECUTION")

    (
        file_name,
        execution,
        process_symbols,
    ) = parallel_algo

    (
        survey_socket,
        server_socket,
    ) = setup_worker(
        workerclass,
        file_name,
        execution,
    )

    survey_socket.send(
        Request(
            task="configure_execution",
            data=execution,
        ).dump()
    )
    response = Response.load(survey_socket.recv())
    assert response.task == "configure_execution"
    assert response.error is None

    survey_socket.send(
        Request(
            task="run_execution",
            data=None,
        ).dump()
    )
    response = Response.load(survey_socket.recv())
    assert response.task == "run_execution"
    assert response.error is None

    process_symbols(
        server_socket,
        "exc_123",
    )
