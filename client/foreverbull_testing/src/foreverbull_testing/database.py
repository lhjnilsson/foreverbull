from datetime import datetime, timedelta

import yfinance
from foreverbull import entity
from sqlalchemy import Column, DateTime, Integer, String, UniqueConstraint, engine, text
from sqlalchemy.orm import declarative_base

Base = declarative_base()


class Asset(Base):
    __tablename__ = "asset"
    symbol = Column("symbol", String(), primary_key=True)
    name = Column("name", String())
    title = Column("title", String())
    asset_type = Column("asset_type", String())


class OHLC(Base):
    __tablename__ = "ohlc"
    id = Column(Integer, primary_key=True)
    symbol = Column(String())
    open = Column(Integer())
    high = Column(Integer())
    low = Column(Integer())
    close = Column(Integer())
    volume = Column(Integer())
    time = Column(DateTime())

    __table_args__ = (UniqueConstraint("symbol", "time", name="symbol_time_uc"),)


def verify(database: engine.Engine, backtest: entity.backtest.Backtest):
    with database.connect() as conn:
        for symbol in backtest.symbols:
            result = conn.execute(
                text("SELECT min(time), max(time) FROM ohlc WHERE symbol = :symbol"),
                {"symbol": symbol},
            )
            res = result.fetchone()
            if res is None:
                print("FAIL TO FETCH MIN MAX")
                return False
            start, end = res
            if start is None or end is None:
                print("START TIME AND TIME IS NONE")
                return False
            if start.date() != backtest.start.date() or end.date() != backtest.end.date():
                print("START TIME AND TIME IS NOT EQUAL")
                print(start.date(), backtest.start.date(), end.date(), backtest.end.date())
                return False
        return True


def populate(database: engine.Engine, backtest: entity.backtest.Backtest):
    with database.connect() as conn:
        for symbol in backtest.symbols:
            feed = yfinance.Ticker(symbol)
            info = feed.info
            asset = entity.finance.Asset(
                symbol=info["symbol"],
                name=info["longName"],
                title=info["shortName"],
                asset_type=info["quoteType"],
            )
            conn.execute(
                text(
                    """INSERT INTO asset (symbol, name, title, asset_type)
                    VALUES (:symbol, :name, :title, :asset_type) ON CONFLICT DO NOTHING"""
                ),
                {"symbol": asset.symbol, "name": asset.name, "title": asset.title, "asset_type": asset.asset_type},
            )
            data = feed.history(start=backtest.start, end=backtest.end + timedelta(days=1))
            for idx, row in data.iterrows():
                time = datetime(idx.year, idx.month, idx.day, idx.hour, idx.minute, idx.second)
                ohlc = entity.finance.OHLC(
                    symbol=symbol,
                    open=row.Open,
                    high=row.High,
                    low=row.Low,
                    close=row.Close,
                    volume=row.Volume,
                    time=time,
                )
                conn.execute(
                    text(
                        """INSERT INTO ohlc (symbol, open, high, low, close, volume, time)
                        VALUES (:symbol, :open, :high, :low, :close, :volume, :time) ON CONFLICT DO NOTHING"""
                    ),
                    {
                        "symbol": ohlc.symbol,
                        "open": ohlc.open,
                        "high": ohlc.high,
                        "low": ohlc.low,
                        "close": ohlc.close,
                        "volume": ohlc.volume,
                        "time": ohlc.time,
                    },
                )
        conn.commit()
