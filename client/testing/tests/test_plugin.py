import os

from datetime import UTC
from datetime import datetime
from typing import cast

from foreverbull_testing.environment import setup_env
from sqlalchemy import Connection

from foreverbull.models.finance import Assets


def test_setup_env_does_not_start_docker(monkeypatch):
    monkeypatch.delenv("TESTCONTAINERS_RYUK_DISABLED", raising=False)
    monkeypatch.delenv("DATABASE_URL", raising=False)
    monkeypatch.delenv("BROKER_HOSTNAME", raising=False)
    setup_env()
    assert "TESTCONTAINERS_RYUK_DISABLED" not in os.environ
    assert "DATABASE_URL" not in os.environ
    assert os.environ.get("BROKER_HOSTNAME") == "127.0.0.1"


def test_fb_env(fb_env):
    assert os.environ.get("BROKER_HOSTNAME") == "127.0.0.1"


def test_namespace_server(namespace_server):
    assert namespace_server == {}


def test_namespace_server_round_trip(fb_env, namespace_server):
    assets = Assets(datetime.now(UTC), cast(Connection, None), [])
    assets.set_metrics("rsi", {"VZ": 5.0, "MCD": 15.0})
    assert namespace_server["rsi"] == {"VZ": 5.0, "MCD": 15.0}
    assert assets.get_metrics("rsi").to_dict() == {"VZ": 5.0, "MCD": 15.0}


def test_fb_database(fb_database):
    engine, verify_or_populate = fb_database
    assert engine is not None
    assert callable(verify_or_populate)
