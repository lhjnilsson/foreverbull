import os

from datetime import date
from datetime import datetime

import pytest

from docker.models.containers import Container
from sqlalchemy import create_engine
from sqlalchemy import text
from testcontainers.core.config import testcontainers_config
from testcontainers.postgres import PostgresContainer

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager

from . import database


def pytest_addoption(parser):
    parser.addoption(
        "--backtest",
        action="store",
        default=None,
        help="Name of a broker backtest to run live algorithm tests against",
    )


def _named_postgres_container(client, name: str):
    for container in client.client.api.containers(all=True, filters={"name": name}):
        names = container.get("Names") or []
        if any(n.rstrip("/").split("/")[-1] == name for n in names):
            return container
    return None


@pytest.fixture(scope="session")
def fb_database():
    postgres = PostgresContainer("postgres:alpine")
    postgres = postgres.with_volume_mapping("postgres_data", "/var/lib/postgresql/data", mode="rw")
    postgres = postgres.with_name("fbull_client_testing_pg")
    client = postgres.get_docker_client()
    existing = _named_postgres_container(client, "fbull_client_testing_pg")
    engine = None
    if existing and existing.get("State") == "running":
        postgres._container = Container(attrs=client.client.api.inspect_container(existing["Id"]))
        try:
            engine = create_engine(postgres.get_connection_url())
            with engine.connect() as conn:
                conn.execute(text("SELECT 1"))
        except Exception:  # noqa: BLE001
            engine = None
    if engine is None:
        if existing:
            client.client.api.remove_container(existing["Id"], force=True)
        postgres._container = None
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
            if type(start) is str:
                start = datetime.strptime(start, "%Y-%m-%d").date()
            if type(end) is str:
                end = datetime.strptime(end, "%Y-%m-%d").date()
            assert start and end and symbols
            entity = backtest_pb2.Backtest(
                periods=[
                    backtest_pb2.Backtest.Period(
                        start_date=pb_utils.from_pydate_to_proto_date(start),
                        end_date=pb_utils.from_pydate_to_proto_date(end),
                        assets=[
                            backtest_pb2.Backtest.Period.Asset(
                                symbol=symbol,
                                enabled=True,
                            )
                            for symbol in symbols
                        ],
                    )
                ]
            )
        if not database.verify(engine, entity):
            database.populate(engine, entity)

    yield engine, verify_or_populate


@pytest.fixture
def asset_manager(fb_database):
    engine, vop = fb_database
    with engine.connect() as conn:
        yield AssetManager(conn, vop)


@pytest.fixture
def portfolio_manager(fb_database):
    engine, _ = fb_database
    with engine.connect() as conn:
        yield PortfolioManager(conn)


@pytest.fixture
def backtest(request):
    name = request.config.getoption("--backtest")
    if not name:
        pytest.skip("pass --backtest <name> to run live backtest tests against a broker")
    pytest.skip(f"live backtest {name!r} requires a running Foreverbull broker")
