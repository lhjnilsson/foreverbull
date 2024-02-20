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
def test_foreverbull(
    spawn_process,
    populate_database,
    ingest_config,
    empty_algo_file,
    execution,
    manual_server,
    session,
    expected_session_type,
):
    populate_database(ingest_config)
    server, socket_config = manual_server
    server.new_execution_data = execution
    server.new_execution_error = None
    server.run_execution_data = execution
    server.run_execution_error = None
    session.port = socket_config.port

    with Foreverbull(session, file_path=empty_algo_file) as foreverbull:
        assert isinstance(foreverbull, expected_session_type)
        assert foreverbull.info
        assert foreverbull.info.type == "worker"
        foreverbull.configure_execution(execution)
        foreverbull.run_execution()
        print("OK")
