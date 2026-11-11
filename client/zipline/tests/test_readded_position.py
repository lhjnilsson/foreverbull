"""Golden run: a held member is dropped from the ETF and added back.

Membership on session t is the half-open interval that contains t.
foreverbull closes a held name on the first session it is not a member,
at that session's close, when auto_close_on_universe_drop is on. Zipline
auto_close_date is not that drop.
"""

import os

from datetime import date
from datetime import datetime
from typing import cast

import pandas as pd
import pytest

from foreverbull_zipline.data_bundles.dividends import no_dividends
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.engine import zero_benchmark_returns
from foreverbull_zipline.universe_close import close_dropped_positions
from foreverbull_zipline.universe_close import order_increases
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


BUNDLE = "readded_member_golden"
AAPL_ISIN = "US0378331005"
XYZ_ISIN = "US9999999999"
START = pd.Timestamp("2023-01-03")
END = pd.Timestamp("2023-03-31")
# The first interval ends at the January month-end list; the February
# month-end list adds XYZ back.
FIRST_DROP = date(2023, 1, 31)
READDED = date(2023, 2, 28)
# The final drop when XYZ leaves again inside the run.
FINAL_DROP = date(2023, 3, 15)
BUY_SESSION = pd.Timestamp("2023-01-04")
XYZ_SHARES = 100
AAPL_SHARES = 10


class _FakeEngine:
    def __init__(self, frames: dict[str, pd.DataFrame]):
        self.frames = frames

    def get_data(self, symbol, from_date, to_date):
        return self.frames[symbol].copy()


def _sessions() -> pd.DatetimeIndex:
    return get_calendar("XNYS").sessions_in_range(START, END)  # type: ignore[arg-type]


def _closes(sessions: pd.DatetimeIndex, first: float, step: float) -> pd.Series:
    return pd.Series([first + step * index for index in range(len(sessions))], index=sessions)


def _bars(closes: pd.Series) -> pd.DataFrame:
    days = pd.DatetimeIndex(closes.index)
    return pd.DataFrame(
        {
            "open": closes.to_list(),
            "high": closes.to_list(),
            "low": closes.to_list(),
            "close": closes.to_list(),
            "volume": [10_000_000] * len(closes),
            "time": [datetime(day.year, day.month, day.day) for day in days],
        }
    )


def _aliases(xyz_intervals: list[tuple[date, date | None]]) -> pd.DataFrame:
    """The load_aliases rows: one per asset_symbol row and membership interval.

    The query orders by ISIN and ticker start, not by membership, so the
    intervals of one ISIN can come back in any order.
    """
    rows = [(AAPL_ISIN, "AAPL", None)] + [(XYZ_ISIN, "XYZ", member_to) for _, member_to in xyz_intervals]
    return pd.DataFrame(
        {
            "isin": [isin for isin, _, _ in rows],
            "symbol": [symbol for _, symbol, _ in rows],
            "from_date": [date(2022, 12, 31)] * len(rows),
            "to_date": [None] * len(rows),
            "member_to": [member_to for _, _, member_to in rows],
        }
    )


def _ingester(sessions: pd.DatetimeIndex, xyz_intervals: list[tuple[date, date | None]]) -> SQLIngester:
    """AAPL a member throughout, XYZ a member in xyz_intervals; both trade on every session."""
    engine = _FakeEngine(
        {
            "AAPL": _bars(_closes(sessions, 50.0, 0.5)),
            "XYZ": _bars(_closes(sessions, 20.0, 0.25)),
            "OEF": _bars(_closes(sessions, 100.0, 1.0)),
        }
    )
    backtest = backtest_pb.Backtest(
        name="sp100",
        benchmark="OEF",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=common_pb.Date(year=2023, month=1, day=3),
                end_date=common_pb.Date(year=2023, month=3, day=31),
                assets=[
                    backtest_pb.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="XYZ", enabled=True),
                ],
            )
        ],
    )
    return SQLIngester(
        backtest,
        engine=engine,
        aliases=_aliases(xyz_intervals),
        events=pd.DataFrame(columns=pd.Index(["symbol", "kind", "on_date", "evidence"])),
        dividends=no_dividends(),
    )


def _members(day: pd.Timestamp, xyz_intervals: list[tuple[date, date | None]]) -> set[str]:
    """AAPL throughout, XYZ on each half-open interval."""
    members = {"AAPL"}
    current = day.date()
    for start, end in xyz_intervals:
        if start <= current and (end is None or current < end):
            members.add("XYZ")
    return members


def _run(
    sessions: pd.DatetimeIndex,
    xyz_intervals: list[tuple[date, date | None]],
    *,
    auto_close: bool = True,
    rebuy: bool = False,
    spread: float = 0.0,
    per_share: float = 0.0,
    also_flatten: bool = False,
    also_increase: bool = False,
) -> pd.DataFrame:
    """Buy AAPL and XYZ on the first session.

    rebuy places another 100 XYZ on 2023-02-28. also_flatten sells the
    position on the first drop session. also_increase tries to buy more
    while XYZ is not a member.
    """
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
        context.set_slippage(slippage.FixedSlippage(spread=spread))
        if per_share:
            context.set_commission(commission.PerShare(cost=per_share, min_trade_cost=0))
        else:
            context.set_commission(commission.NoCommission())
        context.ordered = False

    def handle_data(context, data):
        day = cast(pd.Timestamp, pd.Timestamp(context.get_datetime().date()))
        xyz = context.symbol("XYZ")
        if not context.ordered:
            context.order(context.symbol("AAPL"), AAPL_SHARES)
            context.order(xyz, XYZ_SHARES)
            context.ordered = True
        if rebuy and day == pd.Timestamp(READDED):
            context.order(xyz, XYZ_SHARES)
        members = _members(day, xyz_intervals)
        held = int(context.portfolio.positions[xyz].amount) if xyz in context.portfolio.positions else 0
        if also_flatten and day == pd.Timestamp(FIRST_DROP) and held:
            context.order(xyz, -held)
        if (
            also_increase
            and day == pd.Timestamp("2023-02-01")
            and ("XYZ" in members or not order_increases(held, XYZ_SHARES))
        ):
            context.order(xyz, XYZ_SHARES)
        if auto_close:
            close_dropped_positions(context, data, members, symbol_of=lambda asset: asset.symbol)

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
    result = algorithm.run()
    # Daily results are stamped at the UTC market close; key them by session.
    result.index = pd.DatetimeIndex([pd.Timestamp(stamp.date()) for stamp in pd.DatetimeIndex(result.index)])
    return result


def _ingest_and_run(xyz_intervals: list[tuple[date, date | None]], **run_kwargs) -> tuple[pd.DataFrame, pd.DataFrame]:
    sessions = _sessions()
    ingester = _ingester(sessions, xyz_intervals)
    bundles.register(BUNDLE, ingester, calendar_name="XNYS")
    try:
        bundles.ingest(BUNDLE, os.environ, pd.Timestamp("2023-04-03", tz="UTC"), [], False)
        return pd.DataFrame(ingester._df_metadata), _run(sessions, xyz_intervals, **run_kwargs)
    finally:
        bundles.unregister(BUNDLE)


def _held(row) -> dict[str, int]:
    return {position["sid"].symbol: position["amount"] for position in row["positions"]}


def _xyz_fills(result: pd.DataFrame) -> list[tuple[pd.Timestamp, int, float]]:
    return [
        (cast(pd.Timestamp, day), fill["amount"], fill["price"])
        for day, fills in result["transactions"].items()
        for fill in fills
        if fill["sid"].symbol == "XYZ"
    ]


READDED_AND_HELD = [(date(2022, 12, 31), FIRST_DROP), (READDED, None)]
READDED_AND_DROPPED = [(date(2022, 12, 31), FIRST_DROP), (READDED, FINAL_DROP)]


def _auto_close_is_not_a_membership_drop(metadata: pd.DataFrame) -> None:
    rows = metadata.loc[metadata["symbol"] == "XYZ"]
    assert len(rows) == 1
    closed = pd.Timestamp(rows.iloc[0]["auto_close_date"])
    assert closed == END + pd.Timedelta(days=1)
    assert closed != pd.Timestamp(FIRST_DROP)
    assert closed != pd.Timestamp(FINAL_DROP)


def _cash_change(result: pd.DataFrame, sessions: pd.DatetimeIndex, day: pd.Timestamp) -> float:
    before = sessions[sessions < day][-1]
    return float(result.loc[day]["ending_cash"] - result.loc[before]["ending_cash"])


@pytest.mark.parametrize(
    "xyz_intervals",
    [READDED_AND_HELD, list(reversed(READDED_AND_HELD))],
    ids=["drop-first", "readd-first"],
)
def test_drop_readd_and_a_strategy_rebuy(tmp_path, monkeypatch, xyz_intervals):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    metadata, result = _ingest_and_run(xyz_intervals, rebuy=True)
    sessions = _sessions()
    xyz_closes = _closes(sessions, 20.0, 0.25)
    _auto_close_is_not_a_membership_drop(metadata)

    assert _xyz_fills(result) == [
        (BUY_SESSION, XYZ_SHARES, pytest.approx(xyz_closes[BUY_SESSION])),
        (pd.Timestamp(FIRST_DROP), -XYZ_SHARES, pytest.approx(24.75)),
        (pd.Timestamp("2023-03-01"), XYZ_SHARES, pytest.approx(29.75)),
    ]
    assert _cash_change(result, sessions, pd.Timestamp(FIRST_DROP)) == pytest.approx(2475)
    assert _held(result.loc[pd.Timestamp("2023-01-30")]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}
    assert _held(result.loc[pd.Timestamp(FIRST_DROP)]) == {"AAPL": AAPL_SHARES}
    for day in sessions[(sessions > pd.Timestamp(FIRST_DROP)) & (sessions < pd.Timestamp("2023-03-01"))]:
        assert _held(result.loc[day]) == {"AAPL": AAPL_SHARES}, day
    assert _held(result.loc[pd.Timestamp("2023-03-01")]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}
    assert _held(result.iloc[-1]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}


@pytest.mark.parametrize(
    "xyz_intervals",
    [READDED_AND_DROPPED, list(reversed(READDED_AND_DROPPED))],
    ids=["drop-first", "readd-first"],
)
def test_drop_readd_and_drop_again(tmp_path, monkeypatch, xyz_intervals):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    metadata, result = _ingest_and_run(xyz_intervals, rebuy=True)
    sessions = _sessions()
    xyz_closes = _closes(sessions, 20.0, 0.25)
    _auto_close_is_not_a_membership_drop(metadata)

    assert _xyz_fills(result) == [
        (BUY_SESSION, XYZ_SHARES, pytest.approx(xyz_closes[BUY_SESSION])),
        (pd.Timestamp(FIRST_DROP), -XYZ_SHARES, pytest.approx(24.75)),
        (pd.Timestamp("2023-03-01"), XYZ_SHARES, pytest.approx(29.75)),
        (pd.Timestamp(FINAL_DROP), -XYZ_SHARES, pytest.approx(32.25)),
    ]
    assert _cash_change(result, sessions, pd.Timestamp(FINAL_DROP)) == pytest.approx(3225)
    assert _held(result.loc[pd.Timestamp("2023-03-14")]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}
    assert _held(result.loc[pd.Timestamp(FINAL_DROP)]) == {"AAPL": AAPL_SHARES}
    assert _held(result.iloc[-1]) == {"AAPL": AAPL_SHARES}


@pytest.mark.parametrize(
    "xyz_intervals",
    [READDED_AND_HELD, list(reversed(READDED_AND_HELD))],
    ids=["drop-first", "readd-first"],
)
def test_option_off_holds_through_the_gap(tmp_path, monkeypatch, xyz_intervals):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    metadata, result = _ingest_and_run(xyz_intervals, auto_close=False)
    sessions = _sessions()
    xyz_closes = _closes(sessions, 20.0, 0.25)
    _auto_close_is_not_a_membership_drop(metadata)
    assert _xyz_fills(result) == [(BUY_SESSION, XYZ_SHARES, pytest.approx(xyz_closes[BUY_SESSION]))]
    for day in sessions[sessions >= BUY_SESSION]:
        assert _held(result.loc[day]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}, day


@pytest.mark.parametrize("xyz_intervals", [[(date(2022, 12, 31), FIRST_DROP)]], ids=["no-readd"])
def test_option_off_holds_a_final_drop_with_no_readd(tmp_path, monkeypatch, xyz_intervals):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    metadata, result = _ingest_and_run(xyz_intervals, auto_close=False)
    sessions = _sessions()
    _auto_close_is_not_a_membership_drop(metadata)
    assert _xyz_fills(result) == [(BUY_SESSION, XYZ_SHARES, pytest.approx(20.25))]
    for day in sessions[sessions >= pd.Timestamp(FIRST_DROP)]:
        assert _held(result.loc[day]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}, day


@pytest.mark.parametrize(
    "xyz_intervals",
    [READDED_AND_HELD, list(reversed(READDED_AND_HELD))],
    ids=["drop-first", "readd-first"],
)
def test_costs_apply_to_the_drop_close_and_the_rebuy(tmp_path, monkeypatch, xyz_intervals):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    _, result = _ingest_and_run(xyz_intervals, rebuy=True, spread=0.10, per_share=0.01)
    sessions = _sessions()
    # FixedSlippage sells at close - spread/2 and buys at close + spread/2.
    # PerShare(0.01) costs 1.00 on 100 shares.
    assert _xyz_fills(result)[1] == (pd.Timestamp(FIRST_DROP), -XYZ_SHARES, pytest.approx(24.70))
    assert _cash_change(result, sessions, pd.Timestamp(FIRST_DROP)) == pytest.approx(2469)
    assert _xyz_fills(result)[2] == (pd.Timestamp("2023-03-01"), XYZ_SHARES, pytest.approx(29.80))
    assert _cash_change(result, sessions, pd.Timestamp("2023-03-01")) == pytest.approx(-2981)


@pytest.mark.parametrize(
    "xyz_intervals",
    [READDED_AND_HELD, list(reversed(READDED_AND_HELD))],
    ids=["drop-first", "readd-first"],
)
def test_a_strategy_flatten_on_the_drop_is_not_a_second_sell(tmp_path, monkeypatch, xyz_intervals):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    _, result = _ingest_and_run(xyz_intervals, also_flatten=True)
    sells = [fill for fill in _xyz_fills(result) if fill[1] < 0]
    assert sells == [(pd.Timestamp("2023-02-01"), -XYZ_SHARES, pytest.approx(25.00))]
    assert _held(result.loc[pd.Timestamp(FIRST_DROP)]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}
    assert _held(result.loc[pd.Timestamp("2023-02-01")]) == {"AAPL": AAPL_SHARES}


def test_option_off_can_flatten_a_held_non_member_and_cannot_add(tmp_path, monkeypatch):
    monkeypatch.setenv("ZIPLINE_ROOT", str(tmp_path))
    intervals = [(date(2022, 12, 31), FIRST_DROP)]
    _, held = _ingest_and_run(intervals, auto_close=False, also_flatten=True)
    assert _held(held.loc[pd.Timestamp("2023-02-01")]) == {"AAPL": AAPL_SHARES}
    _, increased = _ingest_and_run(intervals, auto_close=False, also_increase=True)
    assert _xyz_fills(increased) == [(BUY_SESSION, XYZ_SHARES, pytest.approx(20.25))]
    assert _held(increased.iloc[-1]) == {"AAPL": AAPL_SHARES, "XYZ": XYZ_SHARES}
