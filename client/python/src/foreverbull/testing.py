import inspect
import os
import time
from typing import Any

from foreverbull import Foreverbull, broker, entity

try:
    import pytest
    from _pytest.config.argparsing import Parser
except ImportError:
    print("pytest not installed, please install it with `pip install pytest`")
    exit(1)


def pytest_addoption(parser: Parser):
    parser.addoption(
        "--backtest",
        action="store",
    )


class TestingSession:
    def __init__(self, session):
        self.session = session
        self._fb = None

    def __call__(self, module, parameters: list = []) -> Any:
        return Foreverbull(file_path=inspect.getfile(module))


@pytest.fixture(scope="function")
def foreverbull(request):
    session = broker.backtest.run(request.config.getoption("--backtest", skip=True), manual=True)
    while session.port is None:
        time.sleep(0.5)
        session = broker.backtest.get_session(session.id)
        if session.statuses[0].status == entity.backtest.SessionStatusType.FAILED:
            raise Exception(f"Session failed: {session.statuses[-1].error}")
    os.environ["BROKER_SESSION_PORT"] = str(session.port)
    return TestingSession(session)
