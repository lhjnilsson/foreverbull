import os
import tempfile
from datetime import datetime, timedelta, timezone
from multiprocessing import get_start_method, set_start_method

import pandas as pd
import pytest
import yfinance
from sqlalchemy import Column, DateTime, Float, Integer, String, create_engine, text
from sqlalchemy.orm import declarative_base, sessionmaker
from testcontainers.postgres import PostgresContainer
from zipline.data import bundles

import foreverbull_zipline
from foreverbull import entity
from foreverbull.entity.backtest import IngestConfig


@pytest.fixture(scope="session")
def spawn_process():
    method = get_start_method()
    if method != "spawn":
        set_start_method("spawn", force=True)


@pytest.fixture(scope="session")
def execution(postgres_database):
    return entity.backtest.Execution(
        id="test",
        calendar="NYSE",
        start=datetime(2023, 1, 1, 0, 0, 0, 0, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, 0, 0, 0, 0, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
        benchmark="AAPL",
        database=postgres_database.get_connection_url(),
        parameters=None,
        port=5656,
    )


@pytest.fixture(scope="session")
def empty_algo_file():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
import foreverbull
from foreverbull.data import Asset, Portfolio
                
@foreverbull.algo
def empty_algo(asset: Asset, portfolio: Portfolio):
    pass

"""
        )
        f.flush()
        yield f.name


@pytest.fixture(scope="session")
def algo_with_parameters():
    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
import foreverbull
from foreverbull.data import Asset, Portfolio
                
@foreverbull.algo
def algo_with_parameters(asset: Asset, portfolio: Portfolio, low: int = 15, high: int = 25):
    pass

"""
        )
        f.flush()
        yield f.name


@pytest.fixture(scope="session")
def ingest_config():
    return IngestConfig(
        calendar="NYSE",
        start=datetime(2023, 1, 3, tzinfo=timezone.utc),
        end=datetime(2023, 3, 31, tzinfo=timezone.utc),
        symbols=["AAPL", "MSFT", "TSLA"],
    )


@pytest.fixture(scope="session")
def postgres_database():
    with PostgresContainer("postgres:alpine") as postgres:
        yield postgres


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


class Position(Base):
    __tablename__ = "backtest_position"
    id = Column("id", Integer, primary_key=True)
    portfolio_id = Column("portfolio_id", Integer)
    symbol = Column("symbol", String)
    amount = Column("amount", Integer)
    cost_basis = Column("cost_basis", Float)


class Portfolio(Base):
    __tablename__ = "backtest_portfolio"
    id = Column("id", Integer, primary_key=True)
    execution = Column("execution", String)
    date = Column("date", DateTime)
    cash = Column("cash", Float)
    value = Column("value", Float)


@pytest.fixture(scope="session")
def database(postgres_database):
    engine = create_engine(postgres_database.get_connection_url())
    Base.metadata.drop_all(engine)
    Base.metadata.create_all(engine)
    yield engine


@pytest.fixture(scope="session")
def database_session(database):
    sm = sessionmaker(bind=database)
    with sm() as session:
        yield session


@pytest.fixture(scope="session")
def add_asset(database_session):
    def _add_asset(symbol, name, title, asset_type):
        database_session.execute(
            text("INSERT INTO asset (symbol, name, title, asset_type) VALUES (:symbol, :name, :title, :asset_type)"),
            {"symbol": symbol, "name": name, "title": title, "asset_type": asset_type},
        )
        database_session.commit()

    return _add_asset


@pytest.fixture(scope="session")
def add_portfolio(database_session):
    def _add_portfolio(execution, date, cash, value) -> int:
        query = database_session.execute(
            text(
                """INSERT INTO backtest_portfolio (execution, date, cash, value) 
                VALUES (:execution, :date, :cash, :value)
            RETURNING id"""
            ),
            {"execution": execution, "date": date, "cash": cash, "value": value},
        )
        database_session.commit()
        return query.fetchone()[0]

    return _add_portfolio


@pytest.fixture(scope="session")
def add_position(database_session):
    def _add_position(portfolio_id, symbol, amount, cost_basis):
        database_session.execute(
            text(
                """INSERT INTO backtest_position (portfolio_id, symbol, amount, cost_basis) 
                VALUES (:portfolio_id, :symbol, :amount, :cost_basis)"""
            ),
            {"portfolio_id": portfolio_id, "symbol": symbol, "amount": amount, "cost_basis": cost_basis},
        )
        database_session.commit()

    return _add_position


@pytest.fixture(scope="session")
def populate_database(database_session):
    def _populate_database(ic: IngestConfig):
        for symbol in ic.symbols:
            feed = yfinance.Ticker(symbol)
            info = feed.info
            asset = Asset(
                symbol=info["symbol"], name=info["longName"], title=info["shortName"], asset_type=info["quoteType"]
            )
            database_session.add(asset)
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
                database_session.add(ohlc)
        database_session.commit()

    return _populate_database


@pytest.fixture(scope="session")
def foreverbull_bundle(ingest_config, postgres_database, populate_database):
    populate_database(ingest_config)
    os.environ["DATABASE_URL"] = postgres_database.get_connection_url()

    def load_or_create(bundle_name):
        try:
            return bundles.load(bundle_name, os.environ, None)
        except ValueError:
            execution = foreverbull_zipline.Execution()
            execution._ingest(ingest_config)
            return bundles.load(bundle_name, os.environ, None)

    # sanity check
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
