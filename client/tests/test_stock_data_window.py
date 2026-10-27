"""stock_data keeps the last lookback bars inside _STOCK_DATA_WINDOW before as_of.

Bars the broker sent from before the window, or after as_of, stay out of the
frame a strategy reads.
"""

from datetime import UTC
from datetime import datetime
from datetime import timedelta

import pandas
import pytest

from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.models.finance import _STOCK_DATA_LOOKBACK
from foreverbull.models.finance import _STOCK_DATA_WINDOW
from foreverbull.models.finance import _stock_data_since
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb


SYMBOL = "WINDOW"
AS_OF = datetime(2024, 6, 28, 21, tzinfo=UTC)


def _bars(start: datetime, end: datetime, first_close: float) -> pandas.DataFrame:
    times = pandas.bdate_range(start.date(), end.date(), tz=UTC) + pandas.Timedelta(hours=21)
    times = times[times <= end]
    closes = [first_close + i for i in range(len(times))]
    return pandas.DataFrame(
        {"time": times, "open": closes, "high": closes, "low": closes, "close": closes, "volume": 1_000}
    )


def _messages(frame: pandas.DataFrame) -> list[marketdata_pb.OHLC]:
    bars = []
    for row in frame.to_dict("records"):
        when = row["time"].to_pydatetime()
        bars.append(
            marketdata_pb.OHLC(
                symbol=SYMBOL,
                timestamp=pb_utils.to_proto_timestamp(when),
                open=float(row["open"]),
                high=float(row["high"]),
                low=float(row["low"]),
                close=float(row["close"]),
                volume=int(row["volume"]),
            )
        )
    return bars


def _delivered() -> tuple[list[marketdata_pb.OHLC], pandas.DataFrame]:
    since = _stock_data_since(AS_OF)
    old = _bars(since - timedelta(days=400), since - timedelta(days=1), first_close=1.0)
    recent = _bars(AS_OF - timedelta(days=420), AS_OF, first_close=10_000.0)
    later = _bars(AS_OF + timedelta(days=1), AS_OF + timedelta(days=30), first_close=90_000.0)
    edge = marketdata_pb.OHLC(
        symbol=SYMBOL,
        timestamp=pb_utils.to_proto_timestamp(since),
        open=1,
        high=1,
        low=1,
        close=1,
        volume=1,
    )
    return _messages(old) + [edge] + _messages(recent) + _messages(later), recent


def test_window_covers_the_lookback_with_room_for_closures():
    sessions = pandas.bdate_range(_stock_data_since(AS_OF), AS_OF)
    # Weekdays minus about ten exchange holidays a year and a month-long halt.
    holidays = 10 * (_STOCK_DATA_WINDOW.days // 365 + 1)
    assert len(sessions) - holidays - 22 >= _STOCK_DATA_LOOKBACK


def test_stock_data_drops_bars_outside_the_window():
    delivered, recent = _delivered()
    assert len(recent) > _STOCK_DATA_LOOKBACK
    expected = recent["close"].tail(_STOCK_DATA_LOOKBACK).tolist()

    direct = Asset(AS_OF, SYMBOL, delivered).stock_data
    assert direct["close"].tolist() == pytest.approx(expected)
    assert pandas.to_datetime(direct["time"], utc=True).min() > _stock_data_since(AS_OF)
    assert pandas.to_datetime(direct["time"], utc=True).max() <= AS_OF

    batched = Assets(AS_OF, [SYMBOL], delivered).stock_data
    assert batched.loc[SYMBOL]["close"].tolist() == pytest.approx(expected)

    (iterated,) = [asset.stock_data for asset in Assets(AS_OF, [SYMBOL], delivered)]
    pandas.testing.assert_frame_equal(iterated, direct, check_dtype=True)
