from datetime import date
from datetime import datetime

import pandas as pd

from foreverbull_zipline.data_bundles.dividends import no_dividends
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from foreverbull_zipline.data_bundles.foreverbull import _exchange_on
from foreverbull_zipline.data_bundles.foreverbull import group_symbols_by_instrument
from foreverbull_zipline.data_bundles.foreverbull import listing_exchange
from foreverbull_zipline.data_bundles.foreverbull import several_exchanges_line

from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb


AAPL_ISIN = "US0378331005"


def test_listing_exchange_uses_the_stored_venue_and_falls_back():
    assert listing_exchange("XNAS") == "XNAS"
    assert listing_exchange("  XNYS  ") == "XNYS"
    assert listing_exchange(None) == "NASDAQ"
    assert listing_exchange("") == "NASDAQ"
    assert listing_exchange("   ") == "NASDAQ"


def test_two_venues_for_one_isin_are_not_one_exchange():
    closed = [_Identity("US0378331005", "XNAS", date(2020, 1, 1), date(2021, 1, 1))]
    assert _exchange_on(closed, "US0378331005", date(2020, 6, 1)) == "XNAS"
    assert _exchange_on(closed, "US0378331005", date(2021, 1, 1)) is None

    assert several_exchanges_line("aapl-2024", [("AAPL", "US0378331005", date(2024, 1, 2))]) == (
        "Couldn't load data for backtest 'aapl-2024': foreverbull lists AAPL (US0378331005) "
        "on more than one exchange on 2024-01-02, so it can't tell which one to use. "
        "The backtest wasn't saved. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
    )
    assert several_exchanges_line("aapl-2024", [("AAPL", "US0378331005", None)]) == (
        "Couldn't load data for backtest 'aapl-2024': foreverbull lists AAPL (US0378331005) "
        "on more than one exchange, so it can't tell which one to use. "
        "The backtest wasn't saved. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
    )
    assert several_exchanges_line(
        "aapl-2024",
        [
            ("MSFT", "US5949181045", date(2024, 1, 2)),
            ("AAPL", "US0378331005", date(2024, 1, 2)),
        ],
    ) == (
        "Couldn't load data for backtest 'aapl-2024': foreverbull lists 2 symbols on more than one exchange, "
        "so it can't tell which one to use. The backtest wasn't saved.\n"
        "  AAPL (US0378331005) on 2024-01-02\n"
        "  MSFT (US5949181045) on 2024-01-02\n"
        "Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
    )
    assert several_exchanges_line(
        "aapl-2024",
        [("MSFT", "US5949181045", None), ("AAPL", "US0378331005", None)],
    ) == (
        "Couldn't load data for backtest 'aapl-2024': foreverbull lists 2 symbols on more than one exchange, "
        "so it can't tell which one to use. The backtest wasn't saved.\n"
        "  AAPL (US0378331005)\n"
        "  MSFT (US5949181045)\n"
        "Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
    )


class _Identity:
    def __init__(self, isin, mic, valid_from, valid_to):
        self.isin = isin
        self.mic = mic
        self.valid_from = valid_from
        self.valid_to = valid_to


def test_group_keeps_the_listing_exchange():
    aliases = pd.DataFrame(
        {
            "isin": [AAPL_ISIN, AAPL_ISIN],
            "symbol": ["AAPL", "AAPL"],
            "from_date": [date(2020, 1, 1), date(2021, 1, 1)],
            "to_date": [date(2021, 1, 1), None],
            "member_to": [None, None],
            "exchange": ["XNAS", None],
        }
    )
    groups = group_symbols_by_instrument(["AAPL"], aliases)
    assert [alias.exchange for alias in groups[0].aliases] == ["XNAS", None]


class _Bars:
    def __init__(self, frames):
        self.frames = frames

    def get_data(self, symbol, from_date, to_date):
        return self.frames[symbol].copy()


def _bars(days, price):
    return pd.DataFrame(
        {
            "open": [price] * len(days),
            "high": [price] * len(days),
            "low": [price] * len(days),
            "close": [price] * len(days),
            "volume": [100] * len(days),
            "time": [datetime(day.year, day.month, day.day) for day in days],
        }
    )


def _backtest():
    return backtest_pb.Backtest(
        name="exch",
        benchmark="OEF",
        periods=[
            backtest_pb.Backtest.Period(
                start_date=common_pb.Date(year=2023, month=1, day=3),
                end_date=common_pb.Date(year=2023, month=1, day=4),
                assets=[backtest_pb.Backtest.Period.Asset(symbol="AAPL", enabled=True)],
            )
        ],
    )


def _no_events():
    return pd.DataFrame(columns=pd.Index(["symbol", "kind", "on_date", "evidence"]))


def _write(aliases):
    days = [date(2023, 1, 3), date(2023, 1, 4)]
    engine = _Bars({"AAPL": _bars(days, 10.0), "OEF": _bars(days, 20.0)})
    ingester = SQLIngester(_backtest(), engine=engine, aliases=aliases, events=_no_events(), dividends=no_dividends())
    ingester._df_metadata = ingester.create_metadata()
    list(ingester.writer(False))
    return ingester._df_metadata


def test_bundle_uses_a_stored_listing_exchange():
    aliases = pd.DataFrame(
        {
            "isin": [AAPL_ISIN],
            "symbol": ["AAPL"],
            "from_date": [date(2023, 1, 3)],
            "to_date": [None],
            "member_to": [None],
            "exchange": ["XNAS"],
        }
    )
    metadata = _write(aliases)
    aapl = metadata.loc[metadata["symbol"] == "AAPL"].iloc[0]
    benchmark = metadata.loc[metadata["symbol"] == "OEF"].iloc[0]
    assert aapl["exchange"] == "XNAS"
    assert benchmark["exchange"] == "NASDAQ"


def test_bundle_uses_nasdaq_when_the_listing_has_no_exchange():
    metadata = _write(pd.DataFrame())
    assert set(metadata["exchange"].tolist()) == {"NASDAQ"}
