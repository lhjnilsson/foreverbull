from datetime import datetime, timedelta

import yfinance
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull.pb.pb_utils import to_proto_timestamp
from sqlalchemy import Column, DateTime, Integer, String, UniqueConstraint, engine, text
from sqlalchemy.orm import declarative_base

Base = declarative_base()


class Asset(Base):
    __tablename__ = "asset"
    symbol = Column("symbol", String(), primary_key=True)
    name = Column("name", String())


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


def verify(database: engine.Engine, backtest: backtest_pb2.Backtest):
    with database.connect() as conn:
        for symbol in backtest.symbols:
            result = conn.execute(
                text("SELECT min(time), max(time) FROM ohlc WHERE symbol = :symbol"),
                {"symbol": symbol},
            )
            res = result.fetchone()
            if res is None:
                return False
            start, end = res
            if start is None or end is None:
                return False
            if (
                start.date() != backtest.start_date.ToDatetime().date()
                or end.date() != backtest.end_date.ToDatetime().date()
            ):
                return False
        return True


def populate(database: engine.Engine, backtest: backtest_pb2.Backtest):
    with database.connect() as conn:
        for symbol in backtest.symbols:
            feed = yfinance.Ticker(symbol)
            info = feed.info
            asset = finance_pb2.Asset(
                symbol=info["symbol"],
                name=info["longName"],
            )
            conn.execute(
                text(
                    """INSERT INTO asset (symbol, name)
                    VALUES (:symbol, :name) ON CONFLICT DO NOTHING"""
                ),
                {"symbol": asset.symbol, "name": asset.name},
            )
            data = feed.history(
                start=backtest.start_date.ToDatetime(),
                end=backtest.end_date.ToDatetime() + timedelta(days=1),
            )
            for idx, row in data.iterrows():
                time = datetime(
                    idx.year, idx.month, idx.day, idx.hour, idx.minute, idx.second
                )
                ohlc = finance_pb2.OHLC(
                    symbol=symbol,
                    timestamp=to_proto_timestamp(time),
                    open=row.Open,
                    high=row.High,
                    low=row.Low,
                    close=row.Close,
                    volume=int(row.Volume),
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
                        "time": ohlc.timestamp.ToDatetime(),
                    },
                )
        conn.commit()
