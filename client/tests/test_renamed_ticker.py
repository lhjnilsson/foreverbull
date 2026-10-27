"""The client trusts the symbol on each bar.

Alias resolution (FB then META, one ISIN) happens in the broker before the
bars are sent. A bar labeled FB is FB here even when the prices came from
Yahoo's META chart.
"""

from datetime import UTC
from datetime import date
from datetime import datetime
from pathlib import Path

import pandas
import pytest

from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.models import Portfolio
from foreverbull.models.finance import _stock_data_since
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb


YAHOO_META = Path(__file__).parents[1] / "zipline" / "tests" / "testdata" / "yahoo_meta_2021-10-01_2021-11-30.csv"
RENAME = date(2021, 10, 31)
BEFORE = datetime(2021, 10, 15, 21, tzinfo=UTC)
AFTER = datetime(2021, 11, 15, 21, tzinfo=UTC)


def _yahoo_meta() -> pandas.DataFrame:
    frame = pandas.read_csv(YAHOO_META)
    frame["time"] = pandas.to_datetime(frame["time"], utc=True)
    return frame


def _labeled(symbol: str, frame: pandas.DataFrame) -> list[marketdata_pb.OHLC]:
    bars = []
    for row in frame.to_dict("records"):
        when = row["time"].to_pydatetime()
        bars.append(
            marketdata_pb.OHLC(
                symbol=symbol,
                timestamp=pb_utils.to_proto_timestamp(when),
                open=float(row["open"]),
                high=float(row["high"]),
                low=float(row["low"]),
                close=float(row["close"]),
                volume=int(row["volume"]),
            )
        )
    return bars


def _expected(frame: pandas.DataFrame, until: datetime) -> list[float]:
    since = _stock_data_since(until)
    chosen = frame[(frame["time"] > since) & (frame["time"] <= until)]
    return chosen["close"].tolist()


def test_bars_labeled_fb_are_fb_stock_data():
    frame = _yahoo_meta()
    bars = _labeled("FB", frame)
    direct = Asset(BEFORE, "FB", bars).stock_data
    assert direct["close"].tolist() == pytest.approx(_expected(frame, BEFORE))
    assert set(direct["symbol"]) == {"FB"}

    (iterated,) = [asset.stock_data for asset in Assets(BEFORE, ["FB"], bars)]
    assert iterated["close"].tolist() == pytest.approx(_expected(frame, BEFORE))


def test_meta_stock_data_ignores_bars_labeled_fb():
    frame = _yahoo_meta()
    fb = frame[frame["time"].dt.date < RENAME]
    meta = frame[frame["time"].dt.date >= RENAME]
    bars = _labeled("FB", fb) + _labeled("META", meta)
    direct = Asset(AFTER, "META", bars).stock_data
    assert direct["close"].tolist() == pytest.approx(_expected(meta, AFTER))
    assert set(direct["symbol"]) == {"META"}

    (iterated,) = [asset.stock_data for asset in Assets(AFTER, ["META"], bars)]
    assert iterated["close"].tolist() == pytest.approx(_expected(meta, AFTER))


def test_order_value_uses_the_close_on_the_labeled_bar():
    frame = _yahoo_meta()
    bars = _labeled("FB", frame)
    close = _expected(frame, BEFORE)[-1]
    portfolio = Portfolio(
        marketdata_pb.Portfolio(timestamp=pb_utils.to_proto_timestamp(BEFORE), portfolio_value=100_000),
        bars,
    )
    order = portfolio.order_value("FB", 10_000)
    assert order.symbol == "FB"
    assert order.amount == int(10_000 / close)
