from multiprocessing import set_start_method

import parallel

try:
    set_start_method("spawn")
except RuntimeError:
    pass


def test_positive_returns(fb_backtest):
    with fb_backtest(parallel, []) as foreverbull:
        execution = foreverbull.new_backtest_execution()
        foreverbull.run_backtest_execution(execution)
