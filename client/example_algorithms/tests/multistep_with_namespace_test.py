from multiprocessing import set_start_method

from example_algorithms.non_parallel import algo


try:
    set_start_method("spawn")
except RuntimeError:
    pass


def test_positive_returns():
    with algo.backtest_session("nasdaq") as session:
        backtest = session.get_default()
        for period in session.run_execution(
            backtest.start_date,
            backtest.end_date,
            [s for s in backtest.symbols],
        ):
            assert period
