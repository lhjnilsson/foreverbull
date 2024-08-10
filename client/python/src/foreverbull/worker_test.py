import os
from multiprocessing import Event

import pynng
import pytest

from foreverbull import exceptions, worker
from foreverbull.pb.service import service_pb2


@pytest.fixture(scope="function")
def setup_worker():
    survey_address = "ipc:///tmp/worker_pool.ipc"
    survey_socket = pynng.Surveyor0(listen=survey_address)
    survey_socket.recv_timeout = 5000
    survey_socket.send_timeout = 5000
    state_address = "ipc:///tmp/worker_pool_state.ipc"
    state_socket = pynng.Sub0(listen=state_address)
    state_socket.recv_timeout = 5000
    state_socket.send_timeout = 5000
    state_socket.subscribe(b"")

    stop_event = Event()

    def setup(worker: worker.Worker, file_name):
        w = worker(survey_address, state_address, None, stop_event, file_name)
        w.start()
        msg = state_socket.recv()
        assert msg == b"ready"
        return survey_socket

    yield setup

    stop_event.set()
    survey_socket.close()
    state_socket.close()


@pytest.mark.parametrize("instance,expected_error", [])
def test_configure_worker_exceptions(parallel_algo_file, instance, expected_error):
    file_name, configure_response, _ = parallel_algo_file
    w = worker.Worker("ipc:///tmp/worker_pool.ipc", "ipc:///tmp/worker_pool_state.ipc", None, Event(), file_name)
    with (
        pytest.raises(exceptions.ConfigurationError, match=expected_error),
        pynng.Req0(listen="tcp://127.0.0.1:6565"),
    ):
        w.configure_execution(configure_response)


def test_run_worker_unable_to_connect():
    w = worker.Worker("ipc:///tmp/worker_pool.ipc", "ipc:///tmp/worker_pool_state.ipc", None, Event(), "test")
    exit_code = w.run()
    assert exit_code == 1


@pytest.mark.parametrize("workerclass", [worker.WorkerThread, worker.WorkerProcess])
@pytest.mark.parametrize(
    "algo",
    [
        "parallel_algo_file",
        "non_parallel_algo_file",
        "parallel_algo_file_with_parameters",
        "non_parallel_algo_file_with_parameters",
        "multistep_algo_with_namespace",
    ],
)
def test_worker(workerclass: worker.Worker, execution, setup_worker, spawn_process, algo, request):
    if type(workerclass) is worker.WorkerProcess and os.environ.get("THREADED_EXECUTION"):
        pytest.skip("WorkerProcess not supported with THREADED_EXECUTION")

    file_name, configure_response, process_symbols = request.getfixturevalue(algo)
    survey_socket = setup_worker(workerclass, file_name)

    request = service_pb2.Request(task="configure_execution", data=configure_response.SerializeToString())
    survey_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(survey_socket.recv())
    assert response.task == "configure_execution"
    assert response.HasField("error") is False

    request = service_pb2.Request(task="run_execution")
    survey_socket.send(request.SerializeToString())
    response = service_pb2.Response()
    response.ParseFromString(survey_socket.recv())
    assert response.task == "run_execution"
    assert response.HasField("error") is False

    process_symbols()
