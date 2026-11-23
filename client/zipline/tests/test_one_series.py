"""One instrument is one price series.

FB through 2022-06-08 and META from 2022-06-09 are one sid. A META row
from before the rename, on another scale, is not mixed in. The written
FB mapping ends on 2022-06-08.
"""

from datetime import date

import pandas as pd
import pytest

from foreverbull_zipline.data_bundles.dividends import no_dividends
from foreverbull_zipline.data_bundles.foreverbull import AliasSpec
from foreverbull_zipline.data_bundles.foreverbull import DuplicateDay
from foreverbull_zipline.data_bundles.foreverbull import InstrumentGroup
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.data_bundles.foreverbull import repeated_price_day
from foreverbull_zipline.data_bundles.foreverbull import resolve_group_bars
from foreverbull_zipline.data_bundles.foreverbull import symbol_held_line
from foreverbull_zipline.errors import PlainSentence

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb


META = "US30303M1027"
OTHER = "US0000000002"


def _bars(symbol: str, days: list[date], close: float) -> pd.DataFrame:
    return pd.DataFrame(
        {
            "open": [close] * len(days),
            "high": [close + 1] * len(days),
            "low": [close - 1] * len(days),
            "close": [close] * len(days),
            "volume": [1000] * len(days),
            "time": [pd.Timestamp(day) for day in days],
        }
    )


class Frames:
    def __init__(self, frames: dict[str, pd.DataFrame]):
        self.frames = frames

    def get_data(self, symbol, from_date, to_date):
        frame = self.frames[symbol]
        days = pd.to_datetime(frame["time"]).dt.date
        return frame[(days >= from_date) & (days <= to_date)].copy()

    def get_instrument_bars(self, instrument_id, from_date, to_date):
        """The mapped series: FB through 2022-06-08, META from 2022-06-09."""
        del instrument_id
        rename = date(2022, 6, 9)
        fb_end = date(2022, 6, 8)
        fb = self.get_data("FB", from_date, min(to_date, fb_end))
        meta = self.get_data("META", max(from_date, rename), to_date)
        return pd.concat([fb, meta], ignore_index=True)


def _backtest() -> backtest_pb.Backtest:
    return backtest_pb.Backtest(
        name="meta-series",
        benchmark="OEF",
        provider="yahoo",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=common_pb.Date(year=2022, month=6, day=8),
                end_date=common_pb.Date(year=2022, month=6, day=9),
                assets=[
                    backtest_pb.Backtest.Period.Asset(symbol="FB", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="META", enabled=True),
                    backtest_pb.Backtest.Period.Asset(symbol="OEF", enabled=True),
                ],
            )
        ],
    )


_EVENTS = pd.DataFrame(columns=pd.Index(["symbol", "kind", "on_date", "evidence"]))


def test_stooq_fixture_keeps_fb_before_and_meta_from_the_rename():
    engine = Frames(
        {
            "FB": _bars("FB", [date(2022, 6, 8)], 100),
            "META": _bars("META", [date(2022, 6, 7), date(2022, 6, 9)], 40),
            "OEF": _bars("OEF", [date(2022, 6, 8), date(2022, 6, 9)], 50),
        }
    )
    # The pre-rename META row is on another scale and must not appear.
    engine.frames["META"].loc[engine.frames["META"]["time"] == pd.Timestamp(date(2022, 6, 7)), "close"] = 1
    aliases = pd.DataFrame(
        {
            "isin": [META, META],
            "symbol": ["FB", "META"],
            "from_date": [date(2012, 5, 18), date(2022, 6, 9)],
            "to_date": [date(2022, 6, 9), None],
            "member_to": [None, None],
        }
    )
    ingester = SQLIngester(_backtest(), engine=engine, aliases=aliases, events=_EVENTS, dividends=no_dividends())
    ingester._df_metadata = ingester.create_metadata()
    written = dict(ingester.writer(False))
    series = next(frame for frame in written.values() if 100 in set(frame["close"]))
    closes = list(series["close"])
    assert closes == [100, 40]
    assert list(series.index) == [pd.Timestamp("2022-06-08"), pd.Timestamp("2022-06-09")]
    assert 1 not in closes
    rows = ingester._df_metadata
    fb = rows.loc[(rows["symbol"] == "FB") & (rows["end_date"] == pd.Timestamp("2022-06-08"))].iloc[0]
    meta_row = rows.loc[rows["symbol"] == "META"].iloc[0]
    assert int(fb["sid"]) == int(meta_row["sid"])
    assert pd.Timestamp(fb["end_date"]) == pd.Timestamp("2022-06-08")
    assert pd.Timestamp(meta_row["start_date"]) == pd.Timestamp("2022-06-09")


def test_a_bar_outside_every_window_fails_and_names_the_day():
    group = InstrumentGroup(
        aliases=(
            AliasSpec("FB", date(2022, 6, 1), date(2022, 6, 3)),
            AliasSpec("META", date(2022, 6, 8), None),
        ),
        instrument_id=META,
    )
    gap = pd.DataFrame(
        {"open": [1.0], "high": [2.0], "low": [1.0], "close": [1.5], "volume": [10]},
        index=pd.to_datetime(["2022-06-05"]),
    )
    with pytest.raises(PlainSentence, match="META has a price on 2022-06-05 outside its ticker windows"):
        resolve_group_bars(group, {"__instrument__": gap}, date(2022, 6, 1), date(2022, 6, 9))

    # The first alias reaches back to the backtest start, so a bar there is
    # inside the window and is kept.
    early = pd.DataFrame(
        {"open": [1.0], "high": [2.0], "low": [1.0], "close": [1.5], "volume": [10]},
        index=pd.to_datetime(["2022-06-02"]),
    )
    kept, _rows = resolve_group_bars(
        InstrumentGroup(aliases=(AliasSpec("FB", date(2022, 6, 8), date(2022, 6, 10)),), instrument_id=META),
        {"__instrument__": early},
        date(2022, 6, 1),
        date(2022, 6, 9),
    )
    assert kept is not None
    assert list(kept.index) == [pd.Timestamp("2022-06-02")]


def test_a_duplicate_day_names_the_ticker_and_the_day():
    frame = pd.DataFrame({"close": [1.0, 2.0]}, index=pd.to_datetime(["2022-06-09", "2022-06-09"]))
    with pytest.raises(DuplicateDay) as raised:
        repeated_price_day(frame, "META")
    assert str(raised.value) == "META has two prices on 2022-06-09"


def test_two_companies_can_own_fb_on_either_side_of_the_rename(tmp_path):
    """Meta's FB ends 2022-06-08. Another company owns FB from 2022-06-09."""

    class TwoCompanies:
        def get_instrument_bars(self, instrument_id, from_date, to_date):
            if instrument_id == META:
                frame = pd.concat(
                    [_bars("FB", [date(2022, 6, 8)], 100), _bars("META", [date(2022, 6, 9)], 40)],
                    ignore_index=True,
                )
            elif instrument_id == OTHER:
                frame = _bars("FB", [date(2022, 6, 9)], 7)
            else:
                frame = _bars("OEF", [date(2022, 6, 8), date(2022, 6, 9)], 50)
            days = pd.to_datetime(frame["time"]).dt.date
            return frame[(days >= from_date) & (days <= to_date)].copy()

        def get_data(self, symbol, from_date, to_date):
            del symbol
            return self.get_instrument_bars("OEF", from_date, to_date)

    aliases = pd.DataFrame(
        {
            "isin": [META, META, OTHER, OTHER],
            "symbol": ["FB", "META", "OLD", "FB"],
            "from_date": [date(2012, 5, 18), date(2022, 6, 9), date(2012, 5, 18), date(2022, 6, 9)],
            "to_date": [date(2022, 6, 9), None, date(2022, 6, 9), None],
            "member_to": [None, None, None, None],
        }
    )
    ingester = SQLIngester(
        _backtest(), engine=TwoCompanies(), aliases=aliases, events=_EVENTS, dividends=no_dividends()
    )
    ingester._df_metadata = ingester.create_metadata()
    list(ingester.writer(False))
    from zipline.assets.asset_writer import AssetDBWriter

    path = str(tmp_path / "assets.db")
    AssetDBWriter(path).write(equities=ingester._df_metadata)
    from zipline.assets import AssetFinder

    finder = AssetFinder(path)
    meta_asset = finder.lookup_symbol("META", as_of_date=pd.Timestamp("2022-06-09"))
    fb_before = finder.lookup_symbol("FB", as_of_date=pd.Timestamp("2022-06-08"))
    fb_after = finder.lookup_symbol("FB", as_of_date=pd.Timestamp("2022-06-09"))
    assert meta_asset is not None
    assert fb_before is not None
    assert fb_after is not None
    assert fb_before.sid == meta_asset.sid
    assert fb_after.sid != meta_asset.sid


def test_ingest_fails_when_another_instrument_holds_the_ticker():
    engine = Frames(
        {
            "FB": _bars("FB", [date(2022, 6, 8)], 100),
            "OEF": _bars("OEF", [date(2022, 6, 8), date(2022, 6, 9)], 50),
        }
    )
    aliases = pd.DataFrame(
        {
            "isin": [META, OTHER],
            "symbol": ["FB", "FB"],
            "from_date": [date(2012, 5, 18), date(2022, 6, 1)],
            "to_date": [date(2022, 6, 9), date(2022, 6, 10)],
            "member_to": [None, None],
        }
    )
    ingester = SQLIngester(_backtest(), engine=engine, aliases=aliases, events=_EVENTS, dividends=no_dividends())
    ingester._df_metadata = ingester.create_metadata()
    with pytest.raises(PlainSentence, match="FB belongs to another company"):
        list(ingester.writer(False))
    assert symbol_held_line("meta-series", "FB").startswith("Couldn't load data")
