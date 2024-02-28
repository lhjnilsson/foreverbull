from multiprocessing import set_start_method

from example import monkey

from foreverbull import entity

set_start_method("spawn")


def test_positive_returns(foreverbull):
    with foreverbull(monkey, []) as fb:
        fb.configure_execution(entity.backtest.Execution())
        fb.run_execution()
