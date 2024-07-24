import os
import time
from threading import Event, Thread

import pynng
import pytest

from foreverbull import Foreverbull
from foreverbull.pb_gen import service_pb2


@pytest.mark.parametrize(
    "algo",
    [
        "parallel_algo_file",
        "non_parallel_algo_file",
        "parallel_algo_file_with_parameters",
        "non_parallel_algo_file_with_parameters",
    ],
)
def test_foreverbull_over_socket(algo, request):
    file_name, parameters, process_symbols = request.getfixturevalue(algo)

    with Foreverbull(file_name):
        time.sleep(1.0)  # wait for the server to start
        requester = pynng.Req0(dial="tcp://127.0.0.1:5555", block_on_dial=True)
        requester.send_timeout = 1000
        requester.recv_timeout = 1000

        req = service_pb2.Message(task="info")
        requester.send(req.SerializeToString())
        rsp = service_pb2.Message()
        rsp.ParseFromString(requester.recv())
        assert rsp.task == "info"
        assert rsp.HasField("error") is False

        req = service_pb2.Message(task="configure_execution")
        req.data.update(parameters.model_dump())
        requester.send(req.SerializeToString())
        rsp = service_pb2.Message()
        rsp.ParseFromString(requester.recv())
        assert rsp.task == "configure_execution"
        assert rsp.HasField("error") is False

        req = service_pb2.Message(task="run_execution")
        requester.send(req.SerializeToString())
        rsp = service_pb2.Message()
        rsp.ParseFromString(requester.recv())
        assert rsp.task == "run_execution"
        assert rsp.HasField("error") is False

        orders = process_symbols()
        assert len(orders)


@pytest.mark.parametrize(
    "algo",
    [
        "parallel_algo_file",
        "non_parallel_algo_file",
        "parallel_algo_file_with_parameters",
        "non_parallel_algo_file_with_parameters",
    ],
)
def test_foreverbull_manual(execution, algo, request):
    file_name, parameters, process_symbols = request.getfixturevalue(algo)

    stop_event = Event()

    def server():
        sock = pynng.Rep0(listen="tcp://127.0.0.1:6969")
        sock.recv_timeout = 500
        sock.send_timeout = 500
        while not stop_event.is_set():
            try:
                req = service_pb2.Message()
                req.ParseFromString(sock.recv())
                print("----RECEIVED REQUEST", req.task)
                rsp = service_pb2.Message(task=req.task)
                if req.task == "new_execution":
                    rsp.data.update(execution.model_dump())
                elif req.task == "configure_execution":
                    rsp.data.update(parameters.model_dump())
                elif req.task == "run_execution":
                    pass
                elif req.task == "current_period":
                    rsp.data.update({"timestamp": 1})
                else:
                    rsp.error = "Unknown task"
                print("----SENDING RESPONSE", rsp.task)
                sock.send(rsp.SerializeToString())
            except pynng.exceptions.Timeout:
                pass
        sock.close()

    os.environ["BROKER_SESSION_PORT"] = "6969"
    server_thread = Thread(target=server)
    server_thread.start()

    with Foreverbull(file_name) as foreverbull:
        execution = foreverbull.new_backtest_execution()
        run_thread = Thread(target=foreverbull.run_backtest_execution, args=(execution,))
        run_thread.start()

        process_symbols()
        stop_event.set()
        run_thread.join()

    server_thread.join()
