from datetime import UTC
from datetime import datetime
from itertools import pairwise

from foreverbull_testing.plugin import expected_shares
from foreverbull_testing.session import Session

from foreverbull import Algorithm
from foreverbull import Function
from foreverbull.models.finance import Asset
from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio


SYMBOL = "SESS"
AS_OF = datetime(2024, 5, 6, tzinfo=UTC)
CLOSE = 30.0
PORTFOLIO_VALUE = 100_000.0
PERCENT = 0.10


def _bar() -> dict:
    return {
        "symbol": SYMBOL,
        "time": AS_OF,
        "open": CLOSE,
        "high": CLOSE + 1,
        "low": CLOSE - 1,
        "close": CLOSE,
        "volume": 1000,
    }


def _closes(session: Session, symbol: str, as_of: datetime) -> list[float]:
    frame = session.assets([symbol], as_of).stock_data.xs(symbol, level="symbol")
    return [float(close) for close in frame["close"]]


def test_run_order_target_percent_matches_portfolio_formula(session: Session):
    def handle_data(asset: Asset, portfolio: Portfolio) -> None:
        portfolio.order_target_percent(asset.symbol, PERCENT)

    algorithm = Algorithm(functions=[Function(callable=handle_data)])
    times = session.constant(SYMBOL, 1, CLOSE)
    orders = session.run(algorithm, times[-1], PORTFOLIO_VALUE, [])

    assert len(orders) == 1
    assert orders[0].symbol == SYMBOL
    assert orders[0].amount == expected_shares(PORTFOLIO_VALUE, PERCENT, CLOSE)


def test_constant_is_flat_positive_weekday_closes(session: Session):
    price = 12.5
    times = session.constant("FLAT", 5, price)

    assert len(times) == 5
    assert all(stamp.weekday() < 5 for stamp in times)
    frame = session.assets(["FLAT"], times[-1]).stock_data.xs("FLAT", level="symbol")
    assert len(frame) == 5
    closes = [float(close) for close in frame["close"]]
    assert closes == [price] * 5
    assert [float(value) for value in frame["open"]] == closes
    assert [float(value) for value in frame["high"]] == closes
    assert [float(value) for value in frame["low"]] == closes
    assert [int(volume) for volume in frame["volume"]] == [1] * 5


def test_trend_closes_are_monotone(session: Session):
    up_times = session.trend("UP", 6, 10, 20)
    up = _closes(session, "UP", up_times[-1])
    assert up[0] == 10
    assert up[-1] == 20
    assert all(right >= left for left, right in pairwise(up))

    down_times = session.trend("DOWN", 6, 20, 10)
    down = _closes(session, "DOWN", down_times[-1])
    assert down[0] == 20
    assert down[-1] == 10
    assert all(right <= left for left, right in pairwise(down))


def _rolling_mean(closes: list[float], window: int, index: int) -> float:
    return sum(closes[index - window + 1 : index + 1]) / window


def test_cross_up_last_bar_is_first_short_mean_above_long(session: Session):
    short, long = 10, 30
    symbol = "XUP"
    times = session.cross_up(symbol, short, long)

    assert len(times) >= long
    assert all(stamp.weekday() < 5 for stamp in times)
    closes = _closes(session, symbol, times[-1])
    assert len(closes) >= long
    assert all(close > 0 for close in closes)
    previous = len(closes) - 2
    last = len(closes) - 1
    assert previous >= long - 1
    assert _rolling_mean(closes, short, previous) <= _rolling_mean(closes, long, previous)
    assert _rolling_mean(closes, short, last) > _rolling_mean(closes, long, last)
    for index in range(long - 1, last):
        assert _rolling_mean(closes, short, index) <= _rolling_mean(closes, long, index)


def test_assets_and_portfolio_are_worker_types(session: Session):
    session.bars([_bar()])
    assets = session.assets([SYMBOL], AS_OF)
    portfolio = session.portfolio(AS_OF, PORTFOLIO_VALUE, [])

    assert type(assets) is Assets
    assert type(portfolio) is Portfolio
    closes = assets.stock_data.xs(SYMBOL, level="symbol")["close"]
    assert float(closes.iloc[-1]) == CLOSE
