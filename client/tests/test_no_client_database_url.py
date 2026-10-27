"""Backtest and paper workers must not be handed a database URL.

A regression that puts Postgres (or any other database) back on the client
fails here before a strategy process can dial it.
"""

import os
import socket
import tempfile

from pathlib import Path
from threading import Thread

import pynng
import pytest

from protobuf import merge_from_binary

from foreverbull import exceptions
from foreverbull import worker
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb as worker_service_pb
from foreverbull.pb.foreverbull.service.v1 import worker_pb


_SECRET = "super-secret-db-password"
_DATABASE_URL = f"postgres://foreverbull:{_SECRET}@postgres:5432/foreverbull"

_ALGORITHM = b"""
from foreverbull import Algorithm, Function, Asset, Portfolio

def handle(asset: Asset, portfolio: Portfolio):
    portfolio.order(asset.symbol, 1)

algo = Algorithm(functions=[Function(callable=handle)])
"""


def test_client_package_does_not_depend_on_a_database_driver():
    """The installed foreverbull package must not pull a SQL driver for workers."""
    pyproject = Path(__file__).resolve().parents[1] / "pyproject.toml"
    text = pyproject.read_text()
    dependencies = text.split("[project]", 1)[1].split("[build-system]", 1)[0]
    assert "sqlalchemy" not in dependencies
    assert "psycopg" not in dependencies


def test_worker_configure_ignores_a_database_url(caplog, monkeypatch):
    """A URL on ExecutionConfiguration is not a connection string the worker opens."""
    monkeypatch.setenv("BROKER_HOSTNAME", "127.0.0.1")
    namespace = pynng.Rep0(listen="tcp://127.0.0.1:0")
    namespace.recv_timeout = 500
    namespace.send_timeout = 500
    broker = pynng.Rep0(listen="tcp://127.0.0.1:0")
    broker.recv_timeout = 200
    broker.send_timeout = 200
    namespace_port = socket.ntohs(namespace.listeners[0].local_address.port)
    broker_port = socket.ntohs(broker.listeners[0].local_address.port)
    monkeypatch.setenv("NAMESPACE_PORT", str(namespace_port))

    def serve_namespace():
        while True:
            request = worker_service_pb.NamespaceRequest()
            try:
                merge_from_binary(request, namespace.recv())
            except pynng.exceptions.Timeout:
                continue
            except pynng.exceptions.Closed:
                break
            namespace.send(worker_service_pb.NamespaceResponse().to_binary())

    thread = Thread(target=serve_namespace, daemon=True)
    thread.start()

    with tempfile.NamedTemporaryFile(suffix=".py") as handle:
        handle.write(_ALGORITHM)
        handle.flush()
        instance = worker.WorkerInstance(handle.name)
        request = worker_pb.ExecutionConfiguration(
            brokerPort=broker_port,
            namespacePort=namespace_port,
            databaseURL=_DATABASE_URL,
        )
        try:
            instance.configure_execution(request)
        except exceptions.ConfigurationError as exc:
            pytest.fail(f"worker tried to use a database URL: {exc}")
        finally:
            namespace.close()
            broker.close()
            thread.join(timeout=2)

    assert instance.is_configured
    logged = caplog.text
    assert _SECRET not in logged
    assert "postgres://" not in logged
    assert os.environ.get("DATABASE_URL", "") == ""
