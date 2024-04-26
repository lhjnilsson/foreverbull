from threading import Event, Thread

import pynng
import pytest

from foreverbull import Foreverbull, entity
from foreverbull.foreverbull import ManualSession, Session


@pytest.fixture
def manual_server():
    class Server(Thread):
        def __init__(self, host: str, port: int):
            Thread.__init__(self)
            self.stop_event = Event()
            self.socket = pynng.Rep0(listen=f"tcp://{host}:{port}")

            self.new_execution_data = None
            self.new_execution_error = None
            self.run_execution_data = None
            self.run_execution_error = None

        def run(self):
            self.socket.recv_timeout = 100
            while not self.stop_event.is_set():
                try:
                    req = entity.service.Request.load(self.socket.recv())
                    if req.task == "new_execution":
                        self.socket.send(
                            entity.service.Response(
                                task="", data=self.run_execution_data, error=self.run_execution_error
                            ).dump()
                        )
                    elif req.task == "run_execution":
                        self.socket.send(
                            entity.service.Response(
                                task="", data=self.run_execution_data, error=self.run_execution_error
                            ).dump()
                        )
                except pynng.exceptions.Timeout:
                    pass
            self.socket.close()

        def stop(self):
            self.stop_event.set()

    server = Server("127.0.0.1", 6969)
    server.start()
    yield server, entity.service.SocketConfig(host="127.0.0.1", port=6969)
    server.stop()
    server.join()


@pytest.mark.parametrize(
    "session,expected_session_type",
    [
        (
            entity.backtest.Session(
                backtest="test",
                manual=False,
                executions=0,
            ),
            Session,
        ),
        (
            entity.backtest.Session(
                backtest="test",
                manual=True,
                executions=0,
            ),
            ManualSession,
        ),
    ],
)
@pytest.mark.parametrize(
    "algo,expected_info",
    [
        (
            "parallel_algo_file",
            entity.service.Info(
                version="0.0.1",
                parameters=[],
                parallel=True,
            ),
        ),
        (
            "non_parallel_algo_file",
            entity.service.Info(
                version="0.0.1",
                parameters=[],
                parallel=False,
            ),
        ),
        (
            "parallel_algo_file_with_parameters",
            entity.service.Info(
                version="0.0.1",
                parameters=[
                    entity.service.Parameter(key="low", default="15", type="int", value=None),
                    entity.service.Parameter(key="high", default="25", type="int", value=None),
                ],
                parallel=True,
            ),
        ),
        (
            "non_parallel_algo_file_with_parameters",
            entity.service.Info(
                version="0.0.1",
                parameters=[
                    entity.service.Parameter(key="low", default="15", type="int", value=None),
                    entity.service.Parameter(key="high", default="25", type="int", value=None),
                ],
                parallel=False,
            ),
        ),
    ],
)
def test_foreverbull(
    spawn_process,
    execution,
    manual_server,
    process_symbols,
    session,
    expected_session_type,
    algo,
    expected_info,
    request,
):
    server, socket_config = manual_server
    server.new_execution_data = execution
    server.new_execution_error = None
    server.run_execution_data = execution
    server.run_execution_error = None
    session.port = socket_config.port

    algo_file = request.getfixturevalue(algo)

    with (
        Foreverbull(session, file_path=algo_file) as foreverbull,
        pynng.Req0(listen=f"tcp://127.0.0.1:{execution.port}") as socket,
    ):
        assert isinstance(foreverbull, expected_session_type)
        assert foreverbull.info
        assert foreverbull.info == expected_info
        foreverbull.configure_execution(execution)
        foreverbull.run_execution()

        process_symbols(socket, foreverbull.info.parallel)
