from datetime import date
from datetime import datetime

import pandas as pd

from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.data_bundles.foreverbull import group_symbols_by_instrument
from foreverbull_zipline.data_bundles.foreverbull import membership_auto_close_date

from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2


META_ISIN = "US30303M1027"
AAPL_ISIN = "US0378331005"


def _bars(symbol: str, days: list[date], price: float) -> pd.DataFrame:
    return pd.DataFrame(
        {
            "open": [price] * len(days),
            "high": [price] * len(days),
            "low": [price] * len(days),
            "close": [price] * len(days),
            "volume": [100] * len(days),
            "time": [datetime(d.year, d.month, d.day) for d in days],
        }
    )


class FakeEngine:
    def __init__(self, frames: dict[str, pd.DataFrame]):
        self.frames = frames

    def get_data(self, symbol, from_date, to_date):
        return self.frames[symbol].copy()


def _backtest() -> backtest_pb2.Backtest:
    return backtest_pb2.Backtest(
        name="sp100",
        benchmark="OEF",
        periods=[
            backtest_pb2.Backtest.Period(
                start_date=common_pb2.Date(year=2022, month=12, day=1),
                end_date=common_pb2.Date(year=2023, month=2, day=28),
                assets=[
                    backtest_pb2.Backtest.Period.Asset(symbol="FB", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="AAPL", enabled=True),
                    backtest_pb2.Backtest.Period.Asset(symbol="META", enabled=True),
                ],
            )
        ],
    )


def test_membership_auto_close_date_uses_drop_not_last_bar():
    end = pd.Timestamp("2023-12-29")
    assert membership_auto_close_date(end, None) == pd.Timestamp("2023-12-30")
    assert membership_auto_close_date(end, date(2023, 2, 28)) == pd.Timestamp("2023-02-28")


def test_group_symbols_by_instrument_keeps_rename_on_one_sid():
    aliases = pd.DataFrame(
        {
            "isin": [META_ISIN, META_ISIN, AAPL_ISIN],
            "symbol": ["FB", "META", "AAPL"],
            "from_date": [date(2022, 12, 31), date(2023, 1, 31), date(2022, 12, 31)],
            "to_date": [date(2023, 1, 31), None, date(2023, 2, 28)],
            "member_to": [None, None, date(2023, 2, 28)],
        }
    )
    groups = group_symbols_by_instrument(["AAPL", "FB", "META", "OEF"], aliases)
    assert len(groups) == 3
    meta = next(group for group in groups if group.aliases[0].symbol in {"FB", "META"})
    assert [alias.symbol for alias in meta.aliases] == ["FB", "META"]
    assert meta.member_to is None
    aapl = next(group for group in groups if group.aliases[0].symbol == "AAPL")
    assert aapl.member_to == date(2023, 2, 28)
    oef = next(group for group in groups if group.aliases[0].symbol == "OEF")
    assert len(oef.aliases) == 1
    assert oef.member_to is None


def test_sql_ingester_one_sid_for_ticker_change_and_autoclose_on_drop():
    engine = FakeEngine(
        {
            "FB": _bars("FB", [date(2022, 12, 30), date(2023, 1, 30)], 10.0),
            "META": _bars("META", [date(2023, 1, 31), date(2023, 2, 27)], 11.0),
            "AAPL": _bars("AAPL", [date(2022, 12, 30), date(2023, 2, 27)], 12.0),
            "OEF": _bars("OEF", [date(2022, 12, 30), date(2023, 2, 27)], 13.0),
        }
    )
    aliases = pd.DataFrame(
        {
            "isin": [META_ISIN, META_ISIN, AAPL_ISIN],
            "symbol": ["FB", "META", "AAPL"],
            "from_date": [date(2022, 12, 31), date(2023, 1, 31), date(2022, 12, 31)],
            "to_date": [date(2023, 1, 31), None, date(2023, 2, 28)],
            "member_to": [None, None, date(2023, 2, 28)],
        }
    )
    ingester = SQLIngester(_backtest(), engine=engine, aliases=aliases)
    ingester._df_metadata = ingester.create_metadata()
    ingested = list(ingester.writer(False))
    metadata = pd.DataFrame(ingester._df_metadata)

    sids = sorted(set(metadata["sid"].tolist()))
    assert len(sids) == 3
    meta_rows = metadata.loc[metadata["symbol"].isin(["FB", "META"])]
    assert sorted(meta_rows["symbol"].tolist()) == ["FB", "META"]
    assert int(meta_rows["sid"].nunique()) == 1
    assert int(meta_rows["auto_close_date"].nunique()) == 1
    assert pd.Timestamp(meta_rows["auto_close_date"].iloc[0]) == pd.Timestamp("2023-02-28")

    aapl_rows = metadata.loc[metadata["symbol"] == "AAPL"]
    assert len(aapl_rows) == 1
    assert pd.Timestamp(aapl_rows["auto_close_date"].iloc[0]) == pd.Timestamp("2023-02-28")

    meta_sid = int(meta_rows["sid"].iloc[0])
    combined = dict(ingested)[meta_sid]
    assert combined.index.min() == pd.Timestamp("2022-12-30")
    assert combined.index.max() == pd.Timestamp("2023-02-27")


def test_sql_ingester_sets_auto_close_when_no_membership_table():
    engine = FakeEngine(
        {
            "AAPL": _bars("AAPL", [date(2022, 12, 1), date(2023, 2, 28)], 12.0),
            "FB": _bars("FB", [date(2022, 12, 1), date(2023, 2, 28)], 10.0),
            "META": _bars("META", [date(2022, 12, 1), date(2023, 2, 28)], 11.0),
            "OEF": _bars("OEF", [date(2022, 12, 1), date(2023, 2, 28)], 13.0),
        }
    )
    ingester = SQLIngester(_backtest(), engine=engine, aliases=pd.DataFrame())
    ingester._df_metadata = ingester.create_metadata()
    list(ingester.writer(False))
    metadata = pd.DataFrame(ingester._df_metadata)
    assert "autoclose_date" not in metadata.columns
    assert bool(metadata["auto_close_date"].notna().all())
    aapl = metadata.loc[metadata["symbol"] == "AAPL"].iloc[0]
    assert pd.Timestamp(aapl["auto_close_date"]) == pd.Timestamp("2023-03-01")
