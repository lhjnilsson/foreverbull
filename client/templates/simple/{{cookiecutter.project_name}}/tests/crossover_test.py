from {{cookiecutter.project_slug}} import algo 
from multiprocessing import set_start_method
from foreverbull_testing.models import Backtest

try:
    set_start_method("spawn")
except RuntimeError:
    pass


def test_positive_returns(backtest: Backtest):
    backtest.run_execution(algo, backtest.get_default())
