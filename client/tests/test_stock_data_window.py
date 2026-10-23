"""stock_data scans only the bars inside _STOCK_DATA_WINDOW before as_of.

The window is calendar days sized well past _STOCK_DATA_LOOKBACK trading
days, so the frame a strategy reads is the same as an unbounded scan while
the bars stored before the window are never read.
"""

from datetime import UTC
from datetime import datetime
from datetime import timedelta

import pandas
import pytest

from sqlalchemy import event
from sqlalchemy import text

from foreverbull.models import Asset
from foreverbull.models import Assets
from foreverbull.models.finance import _STOCK_DATA_LOOKBACK
from foreverbull.models.finance import _STOCK_DATA_WINDOW
from foreverbull.models.finance import _stock_data_since


PROVIDER = "stock_data_window_test"
SYMBOL = "WINDOW"
AS_OF = datetime(2024, 6, 28, 21, tzinfo=UTC)


def _bars(start: datetime, end: datetime, first_close: float) -> pandas.DataFrame:
    times = pandas.bdate_range(start.date(), end.date(), tz=UTC) + pandas.Timedelta(hours=21)
    times = times[times <= end]
    closes = [first_close + i for i in range(len(times))]
    return pandas.DataFrame(
        {"time": times, "open": closes, "high": closes, "low": closes, "close": closes, "volume": 1_000}
    )


@pytest.fixture
def window_bars(fb_database, monkeypatch):
    """Daily bars well inside the window plus a block older than it."""
    database, _ = fb_database
    monkeypatch.setenv("OHLC_PROVIDER", PROVIDER)
    since = _stock_data_since(AS_OF)
    old = _bars(since - timedelta(days=400), since - timedelta(days=1), first_close=1.0)
    recent = _bars(AS_OF - timedelta(days=420), AS_OF, first_close=10_000.0)
    later = _bars(AS_OF + timedelta(days=1), AS_OF + timedelta(days=30), first_close=90_000.0)
    stored = pandas.concat([old, recent, later], ignore_index=True)
    with database.connect() as conn:
        conn.execute(text("DELETE FROM ohlc WHERE provider = :provider"), {"provider": PROVIDER})
        for bar in stored.to_dict("records"):
            conn.execute(
                text(
                    """INSERT INTO ohlc (provider, symbol, open, high, low, close, volume, time)
                    VALUES (:provider, :symbol, :open, :high, :low, :close, :volume, :time)"""
                ),
                {"provider": PROVIDER, "symbol": SYMBOL, **bar, "time": bar["time"].to_pydatetime()},
            )
        conn.commit()
    yield database, old, recent
    with database.connect() as conn:
        conn.execute(text("DELETE FROM ohlc WHERE provider = :provider"), {"provider": PROVIDER})
        conn.commit()


def test_window_covers_the_lookback_with_room_for_closures():
    sessions = pandas.bdate_range(_stock_data_since(AS_OF), AS_OF)
    # Weekdays minus about ten exchange holidays a year and a month-long halt.
    holidays = 10 * (_STOCK_DATA_WINDOW.days // 365 + 1)
    assert len(sessions) - holidays - 22 >= _STOCK_DATA_LOOKBACK


def test_stock_data_reads_the_same_lookback_without_the_bars_before_the_window(window_bars):
    database, old, recent = window_bars
    assert len(old) > 0
    assert len(recent) > _STOCK_DATA_LOOKBACK
    expected = recent["close"].tail(_STOCK_DATA_LOOKBACK).tolist()
    with database.connect() as conn:
        direct = Asset(AS_OF, conn, SYMBOL).stock_data
        assert direct["close"].tolist() == pytest.approx(expected)
        assert pandas.to_datetime(direct["time"], utc=True).min() > _stock_data_since(AS_OF)

        batched = Assets(AS_OF, conn, [SYMBOL]).stock_data
        assert batched.loc[SYMBOL]["close"].tolist() == pytest.approx(expected)

        (iterated,) = [asset.stock_data for asset in Assets(AS_OF, conn, [SYMBOL])]
        pandas.testing.assert_frame_equal(iterated, direct, check_dtype=True)


def test_stock_data_binds_the_window_start(window_bars):
    database, _, _ = window_bars
    seen: list[tuple[str, dict]] = []

    def before_cursor_execute(conn, cursor, statement, parameters, context, executemany):
        seen.append((statement, dict(parameters)))

    with database.connect() as conn:
        event.listen(conn, "before_cursor_execute", before_cursor_execute)
        try:
            direct = Asset(AS_OF, conn, SYMBOL).stock_data
            batched = Assets(AS_OF, conn, [SYMBOL]).stock_data
        finally:
            event.remove(conn, "before_cursor_execute", before_cursor_execute)
    assert len(direct) == len(batched) == _STOCK_DATA_LOOKBACK
    assert len(seen) == 2
    for statement, parameters in seen:
        assert parameters["since"] == AS_OF - _STOCK_DATA_WINDOW
        assert parameters["as_of"] == AS_OF
        assert "o.time > %(since)s AND o.time <= %(as_of)s" in statement
