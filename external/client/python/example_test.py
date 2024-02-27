from example import monkey

from foreverbull import entity


def test_positive_returns(foreverbull):
    with foreverbull(monkey, []) as fb:
        fb.configure_execution(entity.backtest.Execution())
        fb.run_execution()
