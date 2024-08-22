import inspect
from typing import Any

from foreverbull import Foreverbull


class TestingSession:
    def __init__(self, session):
        self.session = session
        self._fb = None

    def __call__(self, module, parameters: list = []) -> Any:
        return Foreverbull(file_path=inspect.getfile(module))
