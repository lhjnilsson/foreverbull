from datetime import date
from datetime import datetime

import pandas as pd

from foreverbull_zipline.data_bundles.foreverbull import DatabaseEngine
from sqlalchemy import text


def test_database_engine_reads_the_last_day_stamped_at_the_open(fb_database):
    # Yahoo stamps a daily bar at the 14:30 UTC session open. The bar on the
    # period's last day is read; the day after it is not.
    engine, _ = fb_database
    symbol = "LASTDAYBAR"
    days = [datetime(2021, 3, 30, 14, 30), datetime(2021, 3, 31, 14, 30), datetime(2021, 4, 1, 14, 30)]
    with engine.begin() as conn:
        conn.execute(text("DELETE FROM ohlc WHERE symbol = :symbol"), {"symbol": symbol})
        conn.execute(
            text("INSERT INTO asset (symbol, name) VALUES (:symbol, '') ON CONFLICT DO NOTHING"), {"symbol": symbol}
        )
        for day in days:
            conn.execute(
                text(
                    """INSERT INTO ohlc (provider, symbol, open, high, low, close, volume, time)
                    VALUES ('yahoo', :symbol, 1, 1, 1, 1, 1, :time)"""
                ),
                {"symbol": symbol, "time": day},
            )
    reader = DatabaseEngine.__new__(DatabaseEngine)
    reader._engine = engine
    reader.provider = "yahoo"
    try:
        bars = reader.get_data(symbol, date(2021, 3, 30), date(2021, 3, 31))
    finally:
        with engine.begin() as conn:
            conn.execute(text("DELETE FROM ohlc WHERE symbol = :symbol"), {"symbol": symbol})
    assert [pd.Timestamp(t).to_pydatetime() for t in bars["time"]] == days[:2]
