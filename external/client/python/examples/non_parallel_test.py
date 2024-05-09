from multiprocessing import set_start_method

from .non_parallel import handle_data

try:
    set_start_method("spawn")
except RuntimeError:
    pass


def test_positive_returns(foreverbull):
    with foreverbull(handle_data, []) as foreverbull:
        execution = foreverbull.new_backtest_execution()
        foreverbull.run_backtest_execution(execution)
