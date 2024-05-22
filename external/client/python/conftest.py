import os
from datetime import datetime, timedelta, timezone

import pytest
import yfinance
from sqlalchemy import Column, DateTime, Integer, String, UniqueConstraint, create_engine, engine, text
from sqlalchemy.orm import declarative_base
from testcontainers.postgres import PostgresContainer

from foreverbull import entity

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


@pytest.fixture(scope="session")
def backtest_entity():
    return entity.backtest.Backtest(
        name="testing_backtest",
        calendar="NYSE",
        start=datetime(2022, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 12, 29, tzinfo=timezone.utc),
        symbols=[
            "AAPL",
            "AMZN",
            "BAC",
            "BRK-B",
            "CMCSA",
            "CSCO",
            "DIS",
            "GOOG",
            "GOOGL",
            "HD",
            "INTC",
            "JNJ",
            "JPM",
            "KO",
            "MA",
            "META",
            "MRK",
            "MSFT",
            "PEP",
            "PG",
            "TSLA",
            "UNH",
            "V",
            "VZ",
            "WMT",
        ],
    )


@pytest.fixture(scope="session")
def verify_database():
    def _(database: engine.Engine, backtest: entity.backtest.Backtest):
        with database.connect() as conn:
            for symbol in backtest.symbols:
                result = conn.execute(
                    text("SELECT min(time), max(time) FROM ohlc WHERE symbol = :symbol"),
                    {"symbol": symbol},
                )
                start, end = result.fetchone()
                if start is None or end is None:
                    return False
                if start.date() != backtest.start.date() or end.date() != backtest.end.date():
                    return False
            return True

    return _


@pytest.fixture(scope="session")
def populate_database():
    def _(database: engine.Engine, backtest: entity.backtest.Backtest):
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

    return _


@pytest.fixture(scope="session")
def database(backtest_entity: entity.backtest.Backtest, verify_database, populate_database):
    postgres = PostgresContainer("postgres:alpine")
    postgres.with_volume_mapping(
        host=os.path.abspath(".data/test/postgres"), container="/var/lib/postgresql/data", mode="rw"
    )

    with postgres:
        engine = create_engine(postgres.get_connection_url())
        Base.metadata.create_all(engine)
        os.environ["DATABASE_URL"] = postgres.get_connection_url()
        if not verify_database(engine, backtest_entity):
            populate_database(engine, backtest_entity)
        yield engine
