import os

from datetime import date
from datetime import datetime

import pytest

from docker.models.containers import Container
from sqlalchemy import create_engine
from testcontainers.core.config import testcontainers_config
from testcontainers.postgres import PostgresContainer

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest import backtest_pb2

from . import database


@pytest.fixture(scope="session")
def fb_database():
    postgres = PostgresContainer("postgres:alpine")
    postgres = postgres.with_volume_mapping("postgres_data", "/var/lib/postgresql/data", mode="rw")
    postgres = postgres.with_name("fbull_client_testing_pg")
    client = postgres.get_docker_client()
    containers = client.client.api.containers(filters={"name": "fbull_client_testing_pg"})
    if containers:
        postgres._container = Container(attrs=containers[0])
        engine = create_engine(postgres.get_connection_url())
    else:
        testcontainers_config.ryuk_disabled = True
        postgres.start()
        engine = create_engine(postgres.get_connection_url())
        database.Base.metadata.create_all(engine)

    os.environ["DATABASE_URL"] = postgres.get_connection_url()

    def verify_or_populate(
        entity: backtest_pb2.Backtest | None = None,
        start: date | None = None,
        end: datetime | None = None,
        symbols: list[str] | None = None,
    ):
        if entity is None and start is None and end is None and symbols is None:
            raise ValueError("At least one of entity or start, end, and symbols must be provided")
        if not entity:
            assert start and end and symbols
            entity = backtest_pb2.Backtest(
                start_date=pb_utils.from_pydate_to_proto_date(start),
                end_date=pb_utils.from_pydate_to_proto_date(end),
                symbols=symbols,
            )
        if not database.verify(engine, entity):
            database.populate(engine, entity)

    yield engine, verify_or_populate
