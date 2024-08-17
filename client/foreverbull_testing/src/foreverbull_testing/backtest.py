import inspect
import os
import time
from typing import Any

import pytest
from _pytest.config.argparsing import Parser
from foreverbull import Foreverbull, broker, entity


class TestingSession:
    def __init__(self, session):
        self.session = session
        self._fb = None

    def __call__(self, module, parameters: list = []) -> Any:
        return Foreverbull(file_path=inspect.getfile(module))
