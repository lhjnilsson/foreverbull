import inspect

# from foreverbull import Foreverbull
# from foreverbull.foreverbull import BacktestExecution


class TestingSession:
    def __init__(self, session):
        self.session = session
        self._fb = None

    def __call__(self, module, parameters: list = []) -> None:
        return Foreverbull(file_path=inspect.getfile(module))
