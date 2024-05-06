from multiprocessing import set_start_method

from .parallel_example import monkey

set_start_method("spawn")


def test_positive_returns(foreverbull):
    with foreverbull(monkey, []) as foreverbull:
        execution = foreverbull.new_backtest_execution()
        result = foreverbull.run_backtest_execution(execution)
        assert result
