import inspect

from foreverbull import Algorithm


class TestingSession:
    def __init__(self, session):
        self.session = session
        self._fb = None

    def __call__(self, module, parameters: list = []) -> Algorithm:
        return Algorithm.from_file_path(file_path=inspect.getfile(module))
