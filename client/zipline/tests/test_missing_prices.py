"""A missing bar fails the ingest; nothing is filled with the previous close.

Yahoo answers null for a session it has no price for. The broker leaves
that bar out, so the stock's rows skip a session. Rows stored as 0 before
the broker refused non-positive prices are missing bars too. The broker
fails ``fbull backtest create`` on such a session before the ingest
(universe.md section 7.5); the ingester refuses one too, in case a bar
went missing in between.

The META and OEF files are Yahoo's daily chart response for 2021-10-01
to 2021-11-30 (see test_rename_yahoo.py). The golden file is the buy and
hold run below, recorded at 9b3cfb54 before this change: portfolio_value
on the complete bars, and filled_portfolio_value on the gapped bars that
were then filled with the previous close.
"""

import logging
import os

from datetime import date
from pathlib import Path
from typing import Any
from typing import cast

import grpc
import numpy as np
import pandas as pd
import pytest

from foreverbull_zipline.data_bundles.dividends import no_dividends
from foreverbull_zipline.data_bundles.foreverbull import _REASON_BYTE_BOUND
from foreverbull_zipline.data_bundles.foreverbull import MissingSessionsError
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.data_bundles.foreverbull import _fit_reason
from foreverbull_zipline.data_bundles.foreverbull import missing_sessions
from foreverbull_zipline.data_bundles.foreverbull import missing_sessions_status
from foreverbull_zipline.data_bundles.foreverbull import sessions_missing_a_price
from foreverbull_zipline.data_bundles.foreverbull import unpriced_dates
from foreverbull_zipline.service import ingest_failure
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
GOLDEN = TESTDATA / "golden_meta_buy_and_hold_2021-10-01_2021-11-30.csv"
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
    ingester = SQLIngester(
        _backtest(name),
        engine=OHLCRows(frames),
        aliases=pd.DataFrame(),
        events=pd.DataFrame(columns=pd.Index(["symbol", "kind", "on_date", "evidence"])),
        dividends=no_dividends(),
    )
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


def test_missing_sessions_lists_sessions_inside_the_bars_only():
    sessions = pd.DatetimeIndex(["2026-04-30", "2026-05-01", "2026-05-04", "2026-05-05", "2026-05-06", "2026-05-07"])
    bars = pd.DataFrame(
        {"open": [9.0, 11.0], "high": [10.5, 11.5], "low": [8.5, 10.5], "close": [10.0, 11.0], "volume": [100, 200]},
        index=pd.DatetimeIndex(["2026-05-01", "2026-05-06"]),
    )
    # 2026-04-30 is before the first bar and 2026-05-07 after the last.
    assert missing_sessions(bars, sessions) == [pd.Timestamp("2026-05-04"), pd.Timestamp("2026-05-05")]


def test_missing_sessions_leaves_complete_bars_alone():
    sessions = pd.DatetimeIndex(["2026-05-01", "2026-05-04"])
    bars = pd.DataFrame(
        {"open": [1.0, 1.0], "high": [1.0, 1.0], "low": [1.0, 1.0], "close": [1.0, 1.0], "volume": [1, 1]},
        index=sessions,
    )
    assert missing_sessions(bars, sessions) == []


def test_missing_sessions_skips_good_friday():
    # 2024-03-29 is Good Friday: not an XNYS session, so no bar is not missing.
    sessions = get_calendar("XNYS").sessions_in_range(
        pd.Timestamp("2024-03-27"),  # type: ignore[arg-type]
        pd.Timestamp("2024-04-02"),  # type: ignore[arg-type]
    )
    assert pd.Timestamp("2024-03-29") not in sessions
    days = pd.DatetimeIndex(["2024-03-27", "2024-03-28", "2024-04-01", "2024-04-02"])
    bars = pd.DataFrame({"close": [1.0] * len(days)}, index=days)
    assert missing_sessions(bars, sessions) == []


def test_unpriced_dates_names_a_missing_zero_or_negative_price():
    bars = pd.DataFrame(
        {
            "open": [1.0, 0.0, 1.0, 1.0],
            "high": [1.0, 0.0, 1.0, 1.0],
            "low": [1.0, 0.0, -1.0, 1.0],
            "close": [1.0, 0.0, 1.0, np.nan],
            "volume": [1, 0, 1, 1],
        }
    )
    assert list(unpriced_dates(bars)) == [1, 2, 3]


def test_an_unpriced_first_or_last_bar_stays_in_the_span():
    sessions = pd.DatetimeIndex(["2026-05-01", "2026-05-04", "2026-05-05", "2026-05-06"])
    bars = pd.DataFrame(
        {
            "open": [0.0, 1.0, 1.0, 1.0],
            "high": [0.0, 1.0, 1.0, 1.0],
            "low": [0.0, 1.0, 1.0, 1.0],
            "close": [0.0, 1.0, 1.0, np.nan],
        },
        index=sessions,
    )
    # Dropping the edges used to leave only 05-04 and 05-05, so neither bad
    # day was inside the span.
    assert sessions_missing_a_price([bars], bars.iloc[1:3], sessions) == [
        pd.Timestamp("2026-05-01"),
        pd.Timestamp("2026-05-06"),
    ]


def test_an_unpriced_row_that_is_not_a_session_is_still_named():
    sessions = pd.DatetimeIndex(["2026-05-01", "2026-05-04"])
    weekend = pd.Timestamp("2026-05-02")
    bars = pd.DataFrame(
        {"open": [1.0, 0.0], "high": [1.0, 0.0], "low": [1.0, 0.0], "close": [1.0, 0.0]},
        index=pd.DatetimeIndex(["2026-05-01", weekend]),
    )
    assert sessions_missing_a_price([bars], bars.iloc[0:1], sessions) == [weekend]


def test_another_ticker_price_still_covers_the_session():
    sessions = pd.DatetimeIndex(["2026-05-01", "2026-05-04"])
    bad = pd.DataFrame(
        {"open": [0.0, 1.0], "high": [0.0, 1.0], "low": [0.0, 1.0], "close": [0.0, 1.0]},
        index=sessions,
    )
    good = pd.DataFrame(
        {"open": [2.0, 2.0], "high": [2.0, 2.0], "low": [2.0, 2.0], "close": [2.0, 2.0]},
        index=sessions,
    )
    assert sessions_missing_a_price([bad, good], good, sessions) == []


def _golden() -> pd.DataFrame:
    return pd.read_csv(GOLDEN, parse_dates=["date"]).set_index("date")


@pytest.mark.usefixtures("zipline_root")
def test_gap_fails_the_ingest_and_refetched_bars_match_the_golden_run():
    meta, oef = _yahoo_bars("META"), _yahoo_bars("OEF")

    # The run that used to be filled now fails, naming the ticker and every
    # missing session; 0-priced rows count as missing.
    with pytest.raises(MissingSessionsError) as refused:
        _ingest("meta_gap", {"META": _with_gaps(meta), "OEF": oef})
    assert str(refused.value) == "missing prices on trading days: META 2021-10-20,2021-11-03..2021-11-04,2021-11-16"
    assert list(refused.value.gaps) == ["META"]

    # The same run with the gap refetched gives the golden equity curve.
    refetched, _ = _ingest("meta_refetched", {"META": meta, "OEF": oef})
    run = _buy_and_hold(refetched)
    golden = _golden()
    days = pd.DatetimeIndex([pd.Timestamp(day.date()) for day in run.index])
    assert list(days) == list(golden.index)
    assert run["portfolio_value"].to_numpy() == pytest.approx(golden["portfolio_value"].to_numpy(), abs=1e-6)

    # The filled run was off on exactly the four filled sessions.
    differs = golden.index[(golden["portfolio_value"] - golden["filled_portfolio_value"]).abs() > 1e-6]
    assert [day.date() for day in differs] == [*GAP_DAYS, ZERO_DAY]


@pytest.mark.usefixtures("zipline_root")
def test_an_unpriced_first_and_last_bar_fail_the_ingest():
    meta, oef = _yahoo_bars("META").copy(), _yahoo_bars("OEF")
    meta.loc[meta.index[0], ["open", "high", "low", "close"]] = 0
    meta.loc[meta.index[-1], "close"] = np.nan
    with pytest.raises(MissingSessionsError) as refused:
        _ingest("meta_edges", {"META": meta, "OEF": oef})
    assert str(refused.value) == "missing prices on trading days: META 2021-10-01,2021-11-30"
    assert [day.date() for day in refused.value.gaps["META"]] == [date(2021, 10, 1), date(2021, 11, 30)]


@pytest.mark.usefixtures("zipline_root")
def test_bars_that_start_late_or_end_early_still_ingest():
    meta, oef = _yahoo_bars("META"), _yahoo_bars("OEF")
    days = pd.Series(meta["time"]).dt.date
    # A listing after the period start and a delisting before its end:
    # sessions outside the bars are not missing.
    window = cast(pd.DataFrame, meta[(days >= date(2021, 10, 11)) & (days <= date(2021, 11, 19))].copy())
    bundle, _ = _ingest("meta_window", {"META": window, "OEF": oef})
    asset = bundle.asset_finder.lookup_symbol("META", as_of_date=pd.Timestamp("2021-11-19"))
    assert asset.start_date == pd.Timestamp("2021-10-11")
    assert asset.end_date == pd.Timestamp("2021-11-19")


@pytest.mark.usefixtures("zipline_root")
def test_every_gapped_sid_is_named_in_one_error():
    meta, oef = _yahoo_bars("META"), _yahoo_bars("OEF")
    days = pd.Series(oef["time"]).dt.date
    gapped_oef = cast(pd.DataFrame, oef[days != date(2021, 10, 25)].copy())
    with pytest.raises(MissingSessionsError) as refused:
        _ingest("two_gaps", {"META": _with_gaps(meta), "OEF": gapped_oef})
    assert refused.value.gaps.keys() == {"META", "OEF"}
    assert [day.date() for day in refused.value.gaps["OEF"]] == [date(2021, 10, 25)]


def test_missing_sessions_error_caps_symbols_and_ranges():
    sessions = pd.DatetimeIndex(pd.bdate_range("2024-01-02", periods=40))
    isolated = [cast(pd.Timestamp, sessions[index * 2]) for index in range(14)]
    gaps: dict[str, list[pd.Timestamp]] = {"ZZZ": isolated}
    first = cast(pd.Timestamp, sessions[0])
    for number in range(1, 21):
        gaps[f"S{number:02d}"] = [first]
    error = MissingSessionsError(gaps, sessions=sessions, benchmark="ZZZ")
    text = str(error)
    assert text.startswith("missing prices on trading days: ZZZ ")
    assert "and 4 more ranges" in text
    assert text.endswith("; and 1 more symbol")
    assert "S20" not in text
    assert "S19" in text
    assert "S20" in error.full_text
    omitted = cast(pd.Timestamp, sessions[26]).date().isoformat()
    assert omitted in error.full_text
    assert omitted not in text

    run = [cast(pd.Timestamp, day) for day in sessions[:11]]
    one_range = str(MissingSessionsError({"AAA": run}, sessions=sessions))
    assert one_range == (
        f"missing prices on trading days: AAA {run[0].date().isoformat()}..{run[-1].date().isoformat()}"
    )
    assert "more range" not in one_range


def test_a_multi_year_run_is_one_range_under_the_byte_bound():
    sessions = get_calendar("XNYS").sessions_in_range(
        pd.Timestamp("2020-01-01"),  # type: ignore[arg-type]
        pd.Timestamp("2022-12-31"),  # type: ignore[arg-type]
    )
    days = [cast(pd.Timestamp, day) for day in sessions]
    text = str(MissingSessionsError({"AAA": days}, sessions=sessions, benchmark="AAA"))
    first = days[0].date().isoformat()
    last = days[-1].date().isoformat()
    assert text == f"missing prices on trading days: AAA {first}..{last}"
    assert len(text.encode()) < 128
    assert len(missing_sessions_status(text).encode()) <= _REASON_BYTE_BOUND
    records = _gap_fixture().read_text().strip().split("\n\n")
    assert records[0] == text
    many = _capped_many_symbols()
    assert records[1] == many
    assert "and 1 more range" in many
    assert many.endswith("; and 1 more symbol")
    assert "S19" not in many
    assert len(missing_sessions_status(many).encode()) <= _REASON_BYTE_BOUND

    # Twenty symbols, ten kept ranges each, leftovers named. The status text
    # still fits the bound a long run of single dates would not.
    calendar = pd.DatetimeIndex(pd.bdate_range("1990-01-02", periods=80))
    spans = [cast(pd.Timestamp, day) for day in calendar]

    def pairs(count: int) -> list[pd.Timestamp]:
        chosen: list[pd.Timestamp] = []
        for index in range(count):
            chosen.append(spans[index * 3])
            chosen.append(spans[index * 3 + 1])
        return chosen

    many = {f"S{number:02d}": pairs(11) for number in range(21)}
    capped = str(MissingSessionsError(many, sessions=calendar))
    assert ".." in capped
    assert "and 1 more symbol" in capped
    assert len(missing_sessions_status(capped).encode()) <= _REASON_BYTE_BOUND


def _repo_root() -> Path:
    """The checkout that holds go.mod, walked up from this file."""
    here = Path(__file__).resolve()
    for parent in (here, *here.parents):
        if (parent / "go.mod").is_file():
            return parent
    raise RuntimeError(f"no go.mod above {here}")


def _gap_fixture() -> Path:
    return _repo_root() / "pkg/backtest/internal/stream/command/testdata/capped_gap_reason.txt"


def _capped_many_symbols() -> str:
    """Benchmark with 11 ranges, then 20 more symbols, so both leftovers appear."""
    sessions = pd.DatetimeIndex(pd.bdate_range("2024-01-02", periods=40))
    isolated = [cast(pd.Timestamp, sessions[index * 2]) for index in range(11)]
    first = cast(pd.Timestamp, sessions[0])
    gaps: dict[str, list[pd.Timestamp]] = {"ZZZ": isolated}
    for number in range(20):
        gaps[f"S{number:02d}"] = [first]
    return str(MissingSessionsError(gaps, sessions=sessions, benchmark="ZZZ"))


def test_a_status_over_the_bound_drops_whole_symbol_entries():
    parts = [f"S{number:031d} 2024-01-02..2024-01-03 and 9999 more ranges" for number in range(4)]
    one = missing_sessions_status(f"missing prices on trading days: {parts[0]}; and 10 more symbols")
    bound = len(one.encode())
    fitted = _fit_reason(parts, 7, bound=bound)
    status = missing_sessions_status(fitted)
    assert len(status.encode()) <= bound
    assert parts[0] in fitted
    assert parts[1] not in fitted
    assert fitted.endswith("; and 10 more symbols")


def test_worst_case_status_stays_inside_the_byte_bound():
    range_count = 10 + 9999
    calendar = pd.bdate_range("1990-01-02", periods=range_count * 3)
    listed: list[pd.Timestamp] = []
    for index in range(range_count):
        listed.append(cast(pd.Timestamp, calendar[index * 3]))
        listed.append(cast(pd.Timestamp, calendar[index * 3 + 1]))
    gaps: dict[str, list[pd.Timestamp]] = {f"S{number:031d}": listed for number in range(20)}
    one = [cast(pd.Timestamp, calendar[0])]
    for number in range(20, 20 + 9999):
        gaps[f"S{number:031d}"] = one
    error = MissingSessionsError(gaps, sessions=pd.DatetimeIndex(calendar))
    code, details = ingest_failure(error)
    reason, _, cause = details.partition("\nDetails: ")
    assert code == grpc.StatusCode.DATA_LOSS
    assert cause == "MissingSessionsError"
    assert details.count("missing prices on trading days:") == 1
    assert len(details.encode()) <= _REASON_BYTE_BOUND
    assert "and 9999 more ranges" in reason
    assert reason.endswith("; and 9999 more symbols")
    assert f"S{0:031d}" in reason
    assert f"S{19:031d}" in reason
    assert f"S{20:031d}" not in reason
