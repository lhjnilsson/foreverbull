import os

import pytest

from docker.models.containers import Container
from sqlalchemy import create_engine
from testcontainers.core.config import testcontainers_config
from testcontainers.postgres import PostgresContainer

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

    def verify_or_populate(backtest_entity: backtest_pb2.Backtest):
        if not database.verify(engine, backtest_entity):
            database.populate(engine, backtest_entity)

    yield engine, verify_or_populate
