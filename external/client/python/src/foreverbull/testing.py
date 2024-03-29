import argparse
import inspect
import time
from typing import Any

from foreverbull import Foreverbull, broker

try:
    import pytest
except ImportError:
    print("pytest not installed, please install it with `pip install pytest`")
    exit(1)


def pytest_addoption(parser: argparse.ArgumentParser):
    parser.addoption(
        "--backtest",
        action="store",
    )


class TestingSession:
    def __init__(self, session):
        self.session = session
        self._fb = None

    def __call__(self, algo: callable, parameters: [] = []) -> Any:
        return Foreverbull(self.session, file_path=inspect.getfile(algo))


@pytest.fixture(scope="session")
def foreverbull(request):
    session = broker.backtest.run(request.config.getoption("--backtest", skip=True), manual=True)
    while session.port is None:
        time.sleep(0.5)
        session = broker.backtest.get_session(session.id)
    return TestingSession(session)
