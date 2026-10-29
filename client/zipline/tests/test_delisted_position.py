"""Golden run: a held member is delisted mid-run.

The broker leaves a member out of Process from the session after its last
bar. Zipline must close a position in it on that session at the last
close and credit the cash, so the portfolio value does not jump and the
end value is cash plus the other holdings.
"""

import os

from datetime import date
from datetime import datetime

import pandas as pd
import pytest

from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.engine import zero_benchmark_returns
from zipline.data import bundles
from zipline.data.data_portal import DataPortal
from zipline.extensions import load
from zipline.finance import commission
from zipline.finance import metrics
from zipline.finance import slippage
from zipline.finance.blotter import Blotter
from zipline.finance.trading import SimulationParameters
from zipline.utils.calendar_utils import get_calendar

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from zipline import TradingAlgorithm


BUNDLE = "delisted_member_golden"
XYZ_LAST_BAR = pd.Timestamp("2023-01-13")
# 2023-01-16 is Martin Luther King Jr. Day, so the next session is the 17th.
LEFT_OUT_SESSION = pd.Timestamp("2023-01-17")
XYZ_SHARES = 100
AAPL_SHARES = 10


class _FakeEngine:
    def __init__(self, frames: dict[str, pd.DataFrame]):
        self.frames = frames

    def get_data(self, symbol, from_date, to_date):
        return self.frames[symbol].copy()


def _bars(sessions: pd.DatetimeIndex, closes: list[float]) -> pd.DataFrame:
    return pd.DataFrame(
        {
            "open": closes,
            "high": closes,
            "low": closes,
            "close": closes,
            "volume": [10_000_000] * len(closes),
            "time": [datetime(day.year, day.month, day.day) for day in sessions],
        }
    )


def _ingester(sessions: pd.DatetimeIndex) -> SQLIngester:
    xyz_sessions = pd.DatetimeIndex(sessions[sessions <= XYZ_LAST_BAR])
    engine = _FakeEngine(
        {
            "AAPL": _bars(sessions, [50.0 + 0.5 * index for index in range(len(sessions))]),
            "XYZ": _bars(xyz_sessions, [20.0 + 0.25 * index for index in range(len(xyz_sessions))]),
            "OEF": _bars(sessions, [100.0 + index for index in range(len(sessions))]),
        }
    )
    # XYZ stays a member until the month-end list drops it on the 31st.
    aliases = pd.DataFrame(
        {
            "isin": ["US0378331005", "US9999999999"],
            "symbol": ["AAPL", "XYZ"],
            "from_date": [date(2022, 12, 31), date(2022, 12, 31)],
            "to_date": [None, None],
            "member_to": [None, date(2023, 1, 31)],
        }
    )
    backtest = backtest_pb.Backtest(
        name="sp100",
        benchmark="OEF",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=common_pb.Date(year=2023, month=1, day=3),
                end_date=common_pb.Date(year=2023, month=1, day=31),
                assets=[
                    backtest_pb.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="XYZ", enabled=True),
                ],
            )
        ],
    )
    return SQLIngester(backtest, engine=engine, aliases=aliases)


def _run(sessions: pd.DatetimeIndex) -> pd.DataFrame:
    bundle = bundles.load(BUNDLE, os.environ, None)
    calendar = get_calendar("XNYS")
    data_portal = DataPortal(
        bundle.asset_finder,
        trading_calendar=calendar,
        first_trading_day=bundle.equity_minute_bar_reader.first_trading_day,
        equity_minute_reader=bundle.equity_minute_bar_reader,
        equity_daily_reader=bundle.equity_daily_bar_reader,
        adjustment_reader=bundle.adjustment_reader,
    )

    def initialize(context):
        # Fills at the close with no cost, so every dollar is accounted for.
        context.set_slippage(slippage.FixedSlippage(spread=0.0))
        context.set_commission(commission.NoCommission())
        context.aapl = context.symbol("AAPL")
        context.xyz = context.symbol("XYZ")
        context.ordered = False

    def handle_data(context, data):
        if not context.ordered:
            context.order(context.aapl, AAPL_SHARES)
            context.order(context.xyz, XYZ_SHARES)
            context.ordered = True

    algorithm = TradingAlgorithm(
        namespace={},
        data_portal=data_portal,
        trading_calendar=calendar,
        sim_params=SimulationParameters(
            start_session=sessions[0],
            end_session=sessions[-1],
            trading_calendar=calendar,
            data_frequency="daily",
        ),
        metrics_set=metrics.load("default"),
        blotter=load(Blotter, "default"),
        benchmark_returns=zero_benchmark_returns(sessions),
        benchmark_sid=None,
        initialize=initialize,
        handle_data=handle_data,
    )
    return algorithm.run()


def _held(row) -> dict[str, int]:
    return {position["sid"].symbol: position["amount"] for position in row["positions"]}


@pytest.fixture
def delisted_run(tmp_path, monkeypatch):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    sessions = get_calendar("XNYS").sessions_in_range(
        pd.Timestamp("2023-01-03"),  # type: ignore[arg-type]
        pd.Timestamp("2023-01-31"),  # type: ignore[arg-type]
    )
    ingester = _ingester(sessions)
    bundles.register(BUNDLE, ingester, calendar_name="XNYS")
    try:
        bundles.ingest(BUNDLE, os.environ, pd.Timestamp("2023-02-01", tz="UTC"), [], False)
        metadata = pd.DataFrame(ingester._df_metadata)
        yield sessions, metadata, _run(sessions)
    finally:
        bundles.unregister(BUNDLE)


def test_delisted_member_auto_closes_on_the_session_after_its_last_bar(delisted_run):
    _, metadata, _ = delisted_run
    xyz = metadata.loc[metadata["symbol"] == "XYZ"].iloc[0]
    assert pd.Timestamp(xyz["end_date"]) == XYZ_LAST_BAR
    # The day after the last bar, not the month-end drop on the 31st.
    assert pd.Timestamp(xyz["auto_close_date"]) == XYZ_LAST_BAR + pd.Timedelta(days=1)


def test_held_delisted_member_is_sold_at_its_last_close(delisted_run):
    sessions, _, result = delisted_run
    result.index = result.index.tz_localize(None).normalize()
    xyz_closes = [20.0 + 0.25 * index for index in range(len(sessions[sessions <= XYZ_LAST_BAR]))]
    aapl_closes = pd.Series([50.0 + 0.5 * index for index in range(len(sessions))], index=sessions)
    last_bar = result.loc[XYZ_LAST_BAR]
    left_out = result.loc[LEFT_OUT_SESSION]

    assert _held(last_bar) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}
    assert _held(left_out) == {"AAPL": AAPL_SHARES}

    # The close credits the shares at the last stored close, not zero.
    credited = left_out["ending_cash"] - last_bar["ending_cash"]
    assert credited == pytest.approx(XYZ_SHARES * xyz_closes[-1])

    # No jump: the value moves only by what AAPL moved that day.
    aapl_move = AAPL_SHARES * (aapl_closes[LEFT_OUT_SESSION] - aapl_closes[XYZ_LAST_BAR])
    assert left_out["portfolio_value"] - last_bar["portfolio_value"] == pytest.approx(aapl_move)

    # The end value is cash plus the holdings that are left.
    end = result.iloc[-1]
    assert _held(end) == {"AAPL": AAPL_SHARES}
    assert end["portfolio_value"] == pytest.approx(end["ending_cash"] + AAPL_SHARES * aapl_closes.iloc[-1])
