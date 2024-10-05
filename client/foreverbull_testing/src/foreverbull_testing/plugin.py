import os

import pytest
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from sqlalchemy import create_engine
from testcontainers.postgres import PostgresContainer

from . import database


@pytest.fixture(scope="session")
def fb_database():
    postgres = PostgresContainer("postgres:alpine")
    postgres = postgres.with_volume_mapping(
        "postgres_data", "/var/lib/postgresql/data", mode="rw"
    )
    with postgres as postgres:
        engine = create_engine(postgres.get_connection_url())
        database.Base.metadata.create_all(engine)
        os.environ["DATABASE_URL"] = postgres.get_connection_url()

        def verify_or_populate(backtest_entity: backtest_pb2.Backtest):
            if not database.verify(engine, backtest_entity):
                database.populate(engine, backtest_entity)

        yield engine, verify_or_populate
