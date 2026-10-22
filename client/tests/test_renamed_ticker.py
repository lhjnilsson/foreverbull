"""Bars for a renamed ticker (FB then META, one ISIN) through asset_symbol.

Yahoo serves no bars under FB and the whole history under META. The META
bars here are Yahoo's daily chart response for 2021-10-01 to 2021-11-30
(client/zipline/tests/testdata). User code asks for the ticker the
universe lists on its date: FB before the 2021-10-31 rename, META from it.
"""

from datetime import UTC
from datetime import date
from datetime import datetime
from pathlib import Path
from typing import cast

import pandas
import pytest

from sqlalchemy import text

from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.models import Portfolio
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb


YAHOO_META = Path(__file__).parents[1] / "zipline" / "tests" / "testdata" / "yahoo_meta_2021-10-01_2021-11-30.csv"
ISIN = "US30303M1027"
RENAME = date(2021, 10, 31)
BEFORE = datetime(2021, 10, 15, 21, tzinfo=UTC)
AFTER = datetime(2021, 11, 15, 21, tzinfo=UTC)


def _yahoo_meta() -> pandas.DataFrame:
    frame = pandas.read_csv(YAHOO_META)
    frame["time"] = pandas.to_datetime(frame["time"], utc=True)
    return frame


def _store(conn, provider: str, symbol: str, bars: pandas.DataFrame) -> None:
    for bar in bars.to_dict("records"):
        conn.execute(
            text(
                """INSERT INTO ohlc (provider, symbol, open, high, low, close, volume, time)
                VALUES (:provider, :symbol, :open, :high, :low, :close, :volume, :time) ON CONFLICT DO NOTHING"""
            ),
            {
                "provider": provider,
                "symbol": symbol,
                "open": bar["open"],
                "high": bar["high"],
                "low": bar["low"],
                "close": bar["close"],
                "volume": bar["volume"],
                "time": bar["time"].to_pydatetime(),
            },
        )


@pytest.fixture
def renamed(fb_database, monkeypatch):
    """asset_symbol says FB up to the rename and META from it."""
    database, _ = fb_database
    with database.connect() as conn:
        conn.execute(text("DELETE FROM asset_symbol WHERE isin = :isin"), {"isin": ISIN})
        conn.execute(
            text(
                """INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
                (:isin, 'FB', '2021-07-01', '2021-10-31'), (:isin, 'META', '2021-10-31', NULL)"""
            ),
            {"isin": ISIN},
        )
        conn.commit()
    yield database, monkeypatch
    with database.connect() as conn:
        conn.execute(text("DELETE FROM asset_symbol WHERE isin = :isin"), {"isin": ISIN})
        conn.commit()


@pytest.fixture
def yahoo(renamed):
    """Bars as the broker stores them from Yahoo: META only."""
    database, monkeypatch = renamed
    monkeypatch.setenv("OHLC_PROVIDER", "yahoo_rename_test")
    with database.connect() as conn:
        _store(conn, "yahoo_rename_test", "META", _yahoo_meta())
        conn.commit()
    return database


@pytest.fixture
def split(renamed):
    """A provider that lists FB before the rename and META from it."""
    database, monkeypatch = renamed
    monkeypatch.setenv("OHLC_PROVIDER", "split_rename_test")
    bars = _yahoo_meta()
    with database.connect() as conn:
        _store(conn, "split_rename_test", "FB", cast(pandas.DataFrame, bars[bars["time"].dt.date < RENAME]))
        _store(conn, "split_rename_test", "META", cast(pandas.DataFrame, bars[bars["time"].dt.date >= RENAME]))
        conn.commit()
    return database


def _closes(until: datetime) -> list[float]:
    bars = _yahoo_meta()
    return bars[bars["time"] <= until]["close"].tolist()


def test_fb_before_the_rename_reads_the_meta_bars_yahoo_serves(yahoo):
    with yahoo.connect() as conn:
        direct = Asset(BEFORE, conn, "FB").stock_data
        assert direct["close"].tolist() == pytest.approx(_closes(BEFORE))
        assert set(direct["symbol"]) == {"FB"}

        (iterated,) = [asset.stock_data for asset in Assets(BEFORE, conn, ["FB"])]
        assert iterated["close"].tolist() == pytest.approx(_closes(BEFORE))


def test_meta_after_the_rename_reads_fb_bars_from_before_it(split):
    with split.connect() as conn:
        direct = Asset(AFTER, conn, "META").stock_data
        assert direct["close"].tolist() == pytest.approx(_closes(AFTER))

        (iterated,) = [asset.stock_data for asset in Assets(AFTER, conn, ["META"])]
        assert iterated["close"].tolist() == pytest.approx(_closes(AFTER))


def test_order_value_for_fb_before_the_rename_uses_the_yahoo_close(yahoo):
    close = _closes(BEFORE)[-1]
    with yahoo.connect() as conn:
        portfolio = Portfolio(
            marketdata_pb.Portfolio(timestamp=pb_utils.to_proto_timestamp(BEFORE), portfolio_value=100_000),
            conn,
        )
        order = portfolio.order_value("FB", 10_000)
    assert order.symbol == "FB"
    assert order.amount == int(10_000 / close)
