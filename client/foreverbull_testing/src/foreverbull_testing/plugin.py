import inspect
import os
import time
from typing import Any

import pynng
import pytest
import yfinance
from _pytest.config.argparsing import Parser
from foreverbull import Foreverbull, broker, entity
from google.protobuf.timestamp_pb2 import Timestamp
from sqlalchemy import Column, DateTime, Integer, String, UniqueConstraint, create_engine, engine, text
from sqlalchemy.orm import declarative_base
from testcontainers.core.container import DockerContainer
from testcontainers.core.network import Network
from testcontainers.core.waiting_utils import wait_for_logs
from testcontainers.minio import MinioContainer
from testcontainers.nats import NatsContainer
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
    session = broker.backtest.run(request.config.getoption("--backtest", skip=True), manual=True)
    while session.port is None:
        time.sleep(0.5)
        session = broker.backtest.get_session(session.id)
        if session.statuses[0].status == entity.backtest.SessionStatusType.FAILED:
            raise Exception(f"Session failed: {session.statuses[-1].error}")
    os.environ["BROKER_SESSION_PORT"] = str(session.port)
    return TestingSession(session)


@pytest.fixture(scope="session")
def fb_database():
    postgres = PostgresContainer("postgres:alpine")
    postgres = postgres.with_volume_mapping("postgres_data", "/var/lib/postgresql/data", mode="rw")
    with postgres as postgres:
        engine = create_engine(postgres.get_connection_url())
        database.Base.metadata.create_all(engine)
        os.environ["DATABASE_URL"] = postgres.get_connection_url()

        def verify_or_populate(backtest_entity: entity.backtest.Backtest):
            if not database.verify(engine, backtest_entity):
                database.populate(engine, backtest_entity)

        yield engine, verify_or_populate
