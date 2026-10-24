import os
import socket

from datetime import date
from datetime import datetime
from threading import Thread
from typing import cast

import pynng
import pytest

from protobuf import merge_from_binary
from protobuf.wkt import Struct

from foreverbull import broker
from foreverbull.foreverbull import Foreverbull
from foreverbull.models.service import Algorithm
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb as worker_service_pb
from foreverbull_testing.environment import setup_env
from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import Backtest
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.session import Session

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


@pytest.fixture
def fb_env():
    """Process env defaults. Does not start Docker or require a daemon."""
    setup_env()
    yield


@pytest.fixture
def namespace_server():
    """In-process pynng namespace fake. Does not start Docker or query SQL."""
    namespace = {}

    s = pynng.Rep0(listen="tcp://0.0.0.0:0")
    s.recv_timeout = 500
    s.send_timeout = 500
    previous_port = os.environ.get("NAMESPACE_PORT")
    # pynng InAddr.port is network byte order. ntohs is the port clients dial.
    os.environ["NAMESPACE_PORT"] = str(socket.ntohs(s.listeners[0].local_address.port))

    def runner(s, namespace):
        while True:
            request = worker_service_pb.NamespaceRequest()
            try:
                merge_from_binary(request, s.recv())
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            if request.type == worker_service_pb.NamespaceRequestType.GET:
                response = worker_service_pb.NamespaceResponse()
                response.value = Struct.from_python(namespace.get(request.key, {}))
            elif request.type == worker_service_pb.NamespaceRequestType.SET:
                incoming = request.value
                namespace[request.key] = {} if incoming is None else pb_utils.protobuf_struct_to_dict(incoming)
                response = worker_service_pb.NamespaceResponse()
                response.value = Struct.from_python(namespace[request.key])
            else:
                response = worker_service_pb.NamespaceResponse(error="Invalid task")
            s.send(response.to_binary())

    thread = Thread(target=runner, args=(s, namespace))
    thread.start()

    try:
        yield namespace
    finally:
        s.close()
        thread.join()
        if previous_port is None:
            os.environ.pop("NAMESPACE_PORT", None)
        else:
            os.environ["NAMESPACE_PORT"] = previous_port


@pytest.fixture(scope="session")
def fb_database():
    """Postgres Testcontainer plus verify_or_populate for OHLC data.

    Not autouse. Request this only from tests that execute SQL. Reuses a named
    container (`fbull_client_testing_pg`) when one is already running.
    """
    from docker.models.containers import Container
    from sqlalchemy import create_engine
    from sqlalchemy import text
    from testcontainers.core.config import testcontainers_config
    from testcontainers.postgres import PostgresContainer

    # postgres:alpine moved to 18, whose images exit on startup when a volume is
    # mounted at /var/lib/postgresql/data. 17 keeps that path.
    postgres = PostgresContainer("postgres:17-alpine")
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
    database.prepare(engine)

    os.environ["DATABASE_URL"] = postgres.get_connection_url()

    def verify_or_populate(
        entity: backtest_pb.Backtest | None = None,
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
            entity = backtest_pb.Backtest(
                periods=[
                    backtest_pb.Backtest.Period(
                        start_date=pb_utils.from_pydate_to_proto_date(start),
                        end_date=pb_utils.from_pydate_to_proto_date(end),
                        assets=[
                            backtest_pb.Backtest.Period.Asset(
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
def session(fb_database):
    """Synthetic OHLC bars and production Algorithm.process. Not autouse. No price download.

    The yielded Session builds weekday bars with constant, trend, and cross_up.
    replay fills each pending order.amount as a delta at that day's close. The
    result is an order trace for tests, not a broker or Zipline result.
    """
    engine, _ = fb_database
    with engine.connect() as conn:
        yield Session(conn)


class _UnsetAlgorithm:
    """Stand-in until Backtest.run_execution installs the algorithm."""


def _broker_skip_reason(exc: Exception) -> str:
    details = getattr(exc, "details", None)
    message = details() if callable(details) else None
    if not message:
        message = str(exc).strip()
    return str(message) if message else exc.__class__.__name__


def _close_stream(stream) -> None:
    # The session server is already up once a port arrives. Swallow close errors
    # so a stream cancel cannot mask the lookup failure or fail the test.
    closer = getattr(stream, "close", None)
    if not callable(closer):
        return
    try:
        closer()
    except Exception:  # noqa: BLE001
        return


def _started_session(name: str):
    stream = broker.backtest.create_session(name)
    try:
        for msg in stream:
            # Unset optional messages stay None. An empty session has no port yet.
            session = msg.session
            if session is not None and session.port:
                return msg
    finally:
        _close_stream(stream)
    return None


def _backtest(request):
    """Open the optional broker backtest described by request's --backtest option."""
    name = request.config.getoption("--backtest")
    if not name:
        pytest.skip("pass --backtest <name> to run live backtest tests against a broker")

    try:
        entity = broker.backtest.get(name)
    except Exception as exc:  # noqa: BLE001
        pytest.skip(_broker_skip_reason(exc))

    if entity is None or not entity.name:
        pytest.skip(f"backtest {name!r} was not found")

    try:
        started = _started_session(name)
    except Exception as exc:  # noqa: BLE001
        pytest.skip(_broker_skip_reason(exc))

    if started is None or started.session is None:
        pytest.skip(f"backtest {name!r} session did not start")

    # Foreverbull requires a model. The test algorithm replaces it inside run_execution.
    with Foreverbull(cast(Algorithm, _UnsetAlgorithm())).backtest_session(
        entity,
        started.session,
        session_secret=started.session_secret,
    ) as live:
        yield Backtest(live)


@pytest.fixture
def backtest(request):
    """Optional broker backtest. Skips unless --backtest <name> is passed.

    Looks the name up with foreverbull.broker.backtest.get. An unreachable
    broker or a missing name skips with that reason and does not error. A found
    backtest is yielded as models.Backtest, connected with
    broker.backtest.create_session and Foreverbull.backtest_session.
    Backtest.run_execution is the only runner. Does not call trading_session
    or PlaceOrder, and does not set TRADING_ENABLED.
    """
    yield from _backtest(request)
