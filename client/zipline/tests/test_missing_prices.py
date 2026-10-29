"""A missing bar is filled with the previous close, never stored as 0.

Yahoo answers null for a session it has no price for. The broker leaves
that bar out, so the stock's rows skip a session. Zipline's daily bar
writer refuses rows that skip a session, so without a fill one missing
bar fails the whole ingest. Rows stored as 0 before the broker refused
non-positive prices are missing bars too.

The META and OEF files are Yahoo's daily chart response for 2021-10-01
to 2021-11-30 (see test_rename_yahoo.py).
"""

import logging
import os

from datetime import date
from pathlib import Path
from typing import Any
from typing import cast

import numpy as np
import pandas as pd
import pytest

from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.data_bundles.foreverbull import drop_unpriced_bars
from foreverbull_zipline.data_bundles.foreverbull import fill_missing_sessions
from zipline.data import bundles
from zipline.data.data_portal import DataPortal
from zipline.extensions import load
from zipline.finance import metrics
from zipline.finance.blotter import Blotter
from zipline.finance.trading import SimulationParameters
from zipline.utils.calendar_utils import get_calendar

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from zipline import TradingAlgorithm


TESTDATA = Path(__file__).parent / "testdata"
SHARES = 10
# Yahoo null on one session, and on two sessions in a row.
GAP_DAYS = [date(2021, 10, 20), date(2021, 11, 3), date(2021, 11, 4)]
# A row the broker stored as 0 before it refused non-positive prices.
ZERO_DAY = date(2021, 11, 16)


def _yahoo_bars(symbol: str) -> pd.DataFrame:
    frame = pd.read_csv(TESTDATA / f"yahoo_{symbol.lower()}_2021-10-01_2021-11-30.csv")
    frame["time"] = pd.to_datetime(frame["time"], utc=True)
    return frame


def _with_gaps(frame: pd.DataFrame) -> pd.DataFrame:
    days = pd.Series(frame["time"]).dt.date
    gapped = cast(pd.DataFrame, frame[~days.isin(GAP_DAYS)].copy())
    zero = pd.Series(gapped["time"]).dt.date == ZERO_DAY
    gapped.loc[zero, ["open", "high", "low", "close", "volume"]] = 0
    return gapped


class OHLCRows:
    def __init__(self, frames: dict[str, pd.DataFrame]):
        self.frames = frames

    def get_data(self, symbol, from_date, to_date):
        frame = self.frames[symbol]
        days = frame["time"].dt.date
        return frame[(days >= from_date) & (days <= to_date)].copy()


def _backtest(name: str) -> backtest_pb.Backtest:
    return backtest_pb.Backtest(
        name=name,
        benchmark="OEF",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=common_pb.Date(year=2021, month=10, day=1),
                end_date=common_pb.Date(year=2021, month=11, day=30),
                assets=[backtest_pb.Backtest.Period.Asset(symbol="META", enabled=True)],
            )
        ],
    )


def _ingest(name: str, frames: dict[str, pd.DataFrame]) -> tuple[Any, SQLIngester]:
    ingester = SQLIngester(_backtest(name), engine=OHLCRows(frames), aliases=pd.DataFrame())
    bundles.register(name, ingester, calendar_name="XNYS")
    try:
        bundles.ingest(name, os.environ, pd.Timestamp("2021-12-01", tz="UTC"), [], False)
        return bundles.load(name, os.environ, None), ingester
    finally:
        bundles.unregister(name)


def _sessions() -> pd.DatetimeIndex:
    return get_calendar("XNYS").sessions_in_range(
        pd.Timestamp("2021-10-01"),  # type: ignore[arg-type]
        pd.Timestamp("2021-11-30"),  # type: ignore[arg-type]
    )


def _buy_and_hold(bundle) -> pd.DataFrame:
    calendar = get_calendar("XNYS")
    sessions = _sessions()
    ordered: list[bool] = []

    def handle_data(context, data):
        if not ordered:
            context.order(context.symbol("META"), SHARES)
            ordered.append(True)

    data_portal = DataPortal(
        bundle.asset_finder,
        trading_calendar=calendar,
        first_trading_day=bundle.equity_minute_bar_reader.first_trading_day,
        equity_minute_reader=bundle.equity_minute_bar_reader,
        equity_daily_reader=bundle.equity_daily_bar_reader,
        adjustment_reader=bundle.adjustment_reader,
    )
    algorithm = TradingAlgorithm(
        namespace={},
        data_portal=data_portal,
        trading_calendar=calendar,
        sim_params=SimulationParameters(
            start_session=sessions[0],
            end_session=sessions[-1],
            trading_calendar=calendar,
            data_frequency="daily",
            capital_base=10_000,
        ),
        metrics_set=metrics.load("default"),
        blotter=load(Blotter, "default"),
        benchmark_returns=pd.Series(index=sessions, data=0.0),
        benchmark_sid=None,
        handle_data=handle_data,
    )
    logging.getLogger("zipline").setLevel(logging.ERROR)
    return algorithm.run()


@pytest.fixture
def zipline_root(tmp_path, monkeypatch):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))


def test_fill_missing_sessions_uses_previous_close_and_no_volume():
    sessions = pd.DatetimeIndex(["2026-05-01", "2026-05-04", "2026-05-05", "2026-05-06"])
    bars = pd.DataFrame(
        {"open": [9.0, 11.0], "high": [10.5, 11.5], "low": [8.5, 10.5], "close": [10.0, 11.0], "volume": [100, 200]},
        index=pd.DatetimeIndex(["2026-05-01", "2026-05-06"]),
    )
    filled, missing = fill_missing_sessions(bars, sessions)
    assert missing == [pd.Timestamp("2026-05-04"), pd.Timestamp("2026-05-05")]
    assert list(filled.index) == list(sessions)
    for day in missing:
        assert filled.loc[day, ["open", "high", "low", "close"]].tolist() == [10.0, 10.0, 10.0, 10.0]
        assert filled.loc[day, "volume"] == 0
    assert filled["volume"].dtype == bars["volume"].dtype
    assert (filled[["open", "high", "low", "close"]] > 0).all().all()


def test_fill_missing_sessions_leaves_complete_bars_alone():
    sessions = pd.DatetimeIndex(["2026-05-01", "2026-05-04"])
    bars = pd.DataFrame(
        {"open": [1.0, 1.0], "high": [1.0, 1.0], "low": [1.0, 1.0], "close": [1.0, 1.0], "volume": [1, 1]},
        index=sessions,
    )
    filled, missing = fill_missing_sessions(bars, sessions)
    assert missing == []
    assert filled is bars


def test_drop_unpriced_bars_drops_zero_and_negative_prices():
    bars = pd.DataFrame(
        {
            "open": [1.0, 0.0, 1.0, 1.0],
            "high": [1.0, 0.0, 1.0, 1.0],
            "low": [1.0, 0.0, -1.0, 1.0],
            "close": [1.0, 0.0, 1.0, np.nan],
            "volume": [1, 0, 1, 1],
        }
    )
    assert drop_unpriced_bars(bars).index.tolist() == [0]


@pytest.mark.usefixtures("zipline_root")
def test_gap_ingests_and_matches_the_no_gap_baseline():
    meta, oef = _yahoo_bars("META"), _yahoo_bars("OEF")
    baseline, _ = _ingest("meta_no_gap", {"META": meta, "OEF": oef})
    gapped, ingester = _ingest("meta_gap", {"META": _with_gaps(meta), "OEF": oef})

    # The notice names the ticker and the span; 0-priced rows count as missing.
    assert ingester.missing_price_notices() == [
        "Missing prices for META on 4 days (2021-10-20 to 2021-11-16), filled with the previous close. No trades on those days."
    ]

    # Every session has a positive close: no 0 price reaches the backtest.
    sessions = _sessions()
    asset = gapped.asset_finder.lookup_symbol("META", as_of_date=sessions[-1])
    closes = gapped.equity_daily_bar_reader.load_raw_arrays(["close"], sessions[0], sessions[-1], [asset])[0][:, 0]
    assert len(closes) == len(sessions)
    assert bool((closes > 0).all())

    base = _buy_and_hold(baseline)
    gap = _buy_and_hold(gapped)

    # Quant check: no fake -100% day, and Return and drawdown match the
    # baseline. The filled sessions move 0% and the next real session takes
    # the whole move, so End value is identical.
    assert gap["returns"].min() > -0.5
    assert gap["portfolio_value"].iloc[-1] == pytest.approx(base["portfolio_value"].iloc[-1], abs=1e-6)
    assert gap["algorithm_period_return"].iloc[-1] == pytest.approx(base["algorithm_period_return"].iloc[-1], abs=1e-9)
    assert gap["max_drawdown"].iloc[-1] == pytest.approx(base["max_drawdown"].iloc[-1], abs=1e-9)
