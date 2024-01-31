from foreverbull import entity

from . import monkey


def test_positive_returns(foreverbull):
    with foreverbull(monkey, []) as fb:
        fb.configure_execution(entity.backtest.Execution())
        fb.run_execution()
