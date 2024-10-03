import os
import time

import pytest
from _pytest.config.argparsing import Parser
from foreverbull import broker
from foreverbull.pb.foreverbull.backtest import backtest_pb2, session_pb2
from sqlalchemy import create_engine
from testcontainers.postgres import PostgresContainer

from . import database
from .backtest import TestingSession


def pytest_addoption(parser: Parser):
    parser.addoption(
        "--backtest",
        action="store",
    )


@pytest.fixture(scope="function")
def fb_backtest(request):
    session = broker.backtest.create_session(
        request.config.getoption("--backtest", skip=True)
    )
    while session.port is None:
        time.sleep(0.5)
        session = broker.backtest.get_session(session.id)
        if session.statuses[0].status == session_pb2.Session.Status.Status.FAILED:
            raise Exception(f"Session failed: {session.statuses[-1].error}")
    os.environ["BROKER_SESSION_PORT"] = str(session.port)
    return TestingSession(session)


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
