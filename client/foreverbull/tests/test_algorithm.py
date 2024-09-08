import importlib.util
from datetime import datetime

import pytest
from foreverbull import Algorithm, Function, models


class TestAlgorithm:
    @pytest.fixture
    def algorithm(self, parallel_algo_file):
        file_path, _, _ = parallel_algo_file
        spec = importlib.util.spec_from_file_location(
            "",
            file_path,
        )
        source = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(source)
        functions = [Function(callable=models.Algorithm._functions["parallel_algo"]["callable"])]
        return Algorithm(
            functions,
            models.Algorithm._namespaces,
        )

    def test_get_default_no_session(self, algorithm, parallel_algo_file):
        with pytest.raises(RuntimeError, match="No backtest session"):
            algorithm.get_default()

    def test_get_default_with_session(self):
        pass

    def run_execution_no_session(self, algorithm, parallel_algo_file):
        with pytest.raises(RuntimeError, match="No backtest session"):
            algorithm.run_execution(datetime.now(), datetime.now(), [])

    def run_execution_with_session(self):
        pass
