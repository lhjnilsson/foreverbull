from multiprocessing import set_start_method

from example_algorithms.parallel import algo
from foreverbull_testing.models import Backtest


try:
    set_start_method("spawn")
except RuntimeError:
    pass


def test_positive_returns(backtest: Backtest):
    backtest.run_execution(algo, backtest.get_default())
