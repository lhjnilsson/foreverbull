"""Golden strategy metrics: stored scalars against a hand-computed reference.

The reference is plain arithmetic, not empyrical: rf = 0, 252 sessions a
year, sample standard deviation, daily return V_t / V_{t-1} - 1 from the
starting cash, drawdown measured from the starting cash as the first peak. An
undefined ratio is None or NaN here; the engine stores either as unset.
"""

import math

import pandas as pd
import pytest

from foreverbull_zipline.returns import BenchmarkReturns
from foreverbull_zipline.returns import total_return_frame


CAPITAL = 100_000.0
VALUES = [99_000, 99_500, 101_000, 100_200, 102_500, 101_800, 103_000, 102_100, 104_000, 104_200]
DIVIDENDS = [0, 0, 0, 0, 0, 150, 150, 150, 150, 150]
BENCHMARK = [-0.005, 0.003, 0.008, -0.004, 0.012, -0.006, 0.009, -0.007, 0.011, 0.001]
BENCHMARK_PRICE = [b - 0.0001 for b in BENCHMARK]


def _frame(values, dividends, benchmark, benchmark_price):
    sessions = pd.date_range("2024-01-02", periods=len(values), freq="B")
    result = pd.DataFrame(
        {"portfolio_value": [float(v) for v in values], "dividends_earned": [float(d) for d in dividends]},
        index=sessions,
    )
    returns = BenchmarkReturns(
        total=pd.Series(benchmark, index=sessions), price=pd.Series(benchmark_price, index=sessions)
    )
    return total_return_frame(result, CAPITAL, returns)


def test_last_period_matches_the_hand_reference():
    last = _frame(VALUES, DIVIDENDS, BENCHMARK, BENCHMARK_PRICE).iloc[-1]
    assert last["portfolio_value"] == 104_200
    assert last["algorithm_period_return"] == pytest.approx(0.042, abs=1e-12)
    assert last["price_return"] == pytest.approx(0.0405, abs=1e-12)
    assert last["algo_volatility"] == pytest.approx(0.1964768251542068, rel=1e-9)
    assert last["sharpe"] == pytest.approx(5.375627548227965, rel=1e-9)
    assert last["sortino"] == pytest.approx(12.446972772258556, rel=1e-9)
    # The first session's loss from the starting cash is the worst drawdown.
    assert last["max_drawdown"] == pytest.approx(-0.01, abs=1e-12)
    assert last["benchmark_period_return"] == pytest.approx(0.02196595669641166, rel=1e-9)
    assert last["benchmark_price_return"] == pytest.approx(0.020946641415065637, rel=1e-9)
    assert last["beta"] == pytest.approx(1.6389377962238696, rel=1e-9)
    assert last["alpha"] == pytest.approx(0.15895175228278036, rel=1e-9)


def test_every_period_sharpe_and_drawdown_use_only_sessions_so_far():
    frame = _frame(VALUES, DIVIDENDS, BENCHMARK, BENCHMARK_PRICE)
    assert pd.isna(frame["sharpe"].iloc[0])
    assert pd.isna(frame["sortino"].iloc[0])
    assert frame["max_drawdown"].tolist()[:3] == pytest.approx([-0.01, -0.01, -0.01])
    assert frame["sharpe"].iloc[1] == pytest.approx(
        math.sqrt(252) * ((-0.01 + 500 / 99_000) / 2) / abs((-0.01 - 500 / 99_000) / math.sqrt(2)), rel=1e-9
    )


def test_a_tradeless_run_has_zero_return_and_drawdown_and_no_ratios():
    last = _frame([CAPITAL] * 10, [0] * 10, BENCHMARK, BENCHMARK_PRICE).iloc[-1]
    assert last["algorithm_period_return"] == 0.0
    assert last["price_return"] == 0.0
    assert last["max_drawdown"] == 0.0
    assert pd.isna(last["sharpe"])
    assert pd.isna(last["sortino"])


def test_a_run_with_no_losing_session_has_sharpe_but_no_sortino():
    last = _frame([100_500, 101_000, 101_800], [0] * 3, BENCHMARK[:3], BENCHMARK_PRICE[:3]).iloc[-1]
    assert last["max_drawdown"] == 0.0
    assert not pd.isna(last["sharpe"])
    assert pd.isna(last["sortino"])
