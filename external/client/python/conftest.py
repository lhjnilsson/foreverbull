import os
from datetime import datetime, timedelta, timezone
from multiprocessing import get_start_method, set_start_method

import pandas as pd
import pytest
import yfinance
from sqlalchemy import Column, DateTime, Integer, String, create_engine, engine, text
from sqlalchemy.orm import declarative_base
from testcontainers.postgres import PostgresContainer
from zipline.data import bundles

import foreverbull_zipline
from foreverbull import entity
from foreverbull_zipline.entity import IngestConfig


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="function")
def execution():
    return entity.backtest.Execution(
        id="test",
        calendar="NYSE",
        start=datetime(2023, 1, 1, 0, 0, 0, 0, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, 0, 0, 0, 0, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
        benchmark="AAPL",
        database=os.environ.get("DATABASE_URL", ""),
        parameters=None,
        port=5656,
    )


@pytest.fixture(scope="session")
def ingest_config():
    return IngestConfig(
        calendar="NYSE",
        start=datetime(2023, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
    )


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


@pytest.fixture(scope="session")
def database(ingest_config):
    with PostgresContainer("postgres:alpine") as postgres:
        engine = create_engine(postgres.get_connection_url())
        Base.metadata.create_all(engine)
        populate_database(engine, ingest_config)
        os.environ["DATABASE_URL"] = postgres.get_connection_url()
        yield engine


def populate_database(database: engine.Engine, ic: IngestConfig):
    with database.connect() as conn:
        for symbol in ic.symbols:
            feed = yfinance.Ticker(symbol)
            info = feed.info
            asset = Asset(
                symbol=info["symbol"], name=info["longName"], title=info["shortName"], asset_type=info["quoteType"]
            )
            conn.execute(
                text(
                    """INSERT INTO asset (symbol, name, title, asset_type) 
                    VALUES (:symbol, :name, :title, :asset_type)"""
                ),
                {"symbol": asset.symbol, "name": asset.name, "title": asset.title, "asset_type": asset.asset_type},
            )
            data = feed.history(start=ic.start, end=ic.end + timedelta(days=1))
            for idx, row in data.iterrows():
                time = datetime(idx.year, idx.month, idx.day, idx.hour, idx.minute, idx.second)
                ohlc = OHLC(
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
                        VALUES (:symbol, :open, :high, :low, :close, :volume, :time)"""
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


@pytest.fixture(scope="session")
def foreverbull_bundle(ingest_config, database):
    def load_or_create(bundle_name):
        try:
            return bundles.load(bundle_name, os.environ, None)
        except ValueError:
            execution = foreverbull_zipline.Execution()
            execution._ingest(ingest_config)
            return bundles.load(bundle_name, os.environ, None)

    def sanity_check(bundle):
        bundle = load_or_create("foreverbull")
        for symbol in ingest_config.symbols:
            asset = bundle.asset_finder.lookup_symbol(symbol, as_of_date=None)
            assert asset is not None
            start_date = pd.Timestamp(ingest_config.start).normalize().tz_localize(None)
            asset.start_date <= start_date

            end_date = pd.Timestamp(ingest_config.end).normalize().tz_localize(None)
            asset.end_date >= end_date

    bundle = load_or_create("foreverbull")
    sanity_check(bundle)
