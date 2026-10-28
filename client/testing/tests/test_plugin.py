import os

from datetime import UTC
from datetime import datetime
from types import SimpleNamespace

import pytest

from foreverbull_testing.environment import setup_env
from foreverbull_testing.models import Backtest
from foreverbull_testing.plugin import _backtest

from foreverbull.models.finance import Assets
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import session_pb


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
    assets = Assets(datetime.now(UTC), [])
    assets.set_metrics("rsi", {"VZ": 5.0, "MCD": 15.0})
    assert namespace_server["rsi"] == {"VZ": 5.0, "MCD": 15.0}
    assert assets.get_metrics("rsi").to_dict() == {"VZ": 5.0, "MCD": 15.0}


def test_fb_database(fb_database):
    engine, verify_or_populate = fb_database
    assert engine is not None
    assert callable(verify_or_populate)


def _backtest_request(name: str | None):
    def getoption(option):
        assert option == "--backtest"
        return name

    return SimpleNamespace(config=SimpleNamespace(getoption=getoption))


def _advance(name: str | None):
    # The helper yields, so the body runs on next(), including pytest.skip.
    return next(_backtest(_backtest_request(name)))


class _LookupError(Exception):
    def __init__(self, reason: str):
        super().__init__("rpc failed")
        self._reason = reason

    def details(self):
        return self._reason


def test_backtest_fixture_skips_without_option(monkeypatch):
    def fail_get(name):
        raise AssertionError(f"broker get should not be called for {name}")

    monkeypatch.setattr("foreverbull.broker.backtest.get", fail_get)
    with pytest.raises(pytest.skip.Exception, match=r"pass --backtest <name>"):
        _advance(None)


def test_backtest_fixture_skips_when_broker_unreachable(monkeypatch):
    def fail_get(name):
        raise OSError(f"connection refused for {name}")

    monkeypatch.setattr("foreverbull.broker.backtest.get", fail_get)
    monkeypatch.setattr(
        "foreverbull.broker.backtest.create_session",
        lambda name: (_ for _ in ()).throw(AssertionError("create_session should not be called")),
    )
    with pytest.raises(pytest.skip.Exception, match="connection refused"):
        _advance("demo")


def test_backtest_fixture_skips_when_backtest_missing(monkeypatch):
    def fail_get(name):
        raise _LookupError(f"error getting backtest {name}: no rows in result set")

    monkeypatch.setattr("foreverbull.broker.backtest.get", fail_get)
    with pytest.raises(pytest.skip.Exception, match="no rows in result set"):
        _advance("missing")


def test_backtest_fixture_skips_when_session_does_not_start(monkeypatch):
    monkeypatch.setattr(
        "foreverbull.broker.backtest.get",
        lambda name: backtest_pb.Backtest(name=name),
    )
    monkeypatch.setattr("foreverbull.broker.backtest.create_session", lambda name: iter(()))
    with pytest.raises(pytest.skip.Exception, match="session did not start"):
        _advance("demo")


def test_backtest_fixture_skips_when_create_session_fails(monkeypatch):
    def fail(name):
        raise OSError(f"session failed for {name}")

    monkeypatch.setattr(
        "foreverbull.broker.backtest.get",
        lambda name: backtest_pb.Backtest(name=name),
    )
    monkeypatch.setattr("foreverbull.broker.backtest.create_session", fail)
    with pytest.raises(pytest.skip.Exception, match="session failed"):
        _advance("demo")


def test_backtest_fixture_skips_when_backtest_name_empty(monkeypatch):
    monkeypatch.setattr("foreverbull.broker.backtest.get", lambda name: backtest_pb.Backtest())
    monkeypatch.setattr(
        "foreverbull.broker.backtest.create_session",
        lambda name: (_ for _ in ()).throw(AssertionError("create_session should not be called")),
    )
    with pytest.raises(pytest.skip.Exception, match="was not found"):
        _advance("missing")


def test_backtest_fixture_yields_broker_backtest(monkeypatch):
    entity = backtest_pb.Backtest(name="demo")
    ready = session_pb.Session(id="sess-1", backtest="demo", port=27001)
    messages = [
        backtest_service_pb.CreateSessionResponse(session_secret="early-secret"),
        backtest_service_pb.CreateSessionResponse(session=ready, session_secret="port-frame-secret"),
    ]
    calls = {"get": [], "create_session": [], "backtest_session": [], "trading_session": []}
    trading_before = os.environ.get("TRADING_ENABLED")

    def get(name):
        calls["get"].append(name)
        return entity

    def create_session(name):
        calls["create_session"].append(name)
        yield from messages

    class _Live:
        def __init__(self, backtest):
            self.backtest = backtest

    class _Ctx:
        def __init__(self, live):
            self.live = live
            self.exited = False

        def __enter__(self):
            return self.live

        def __exit__(self, exc_type, exc, tb):
            self.exited = True
            return False

    class _FB:
        def __init__(self, model):
            self.model = model

        def backtest_session(self, backtest, session, broker_hostname=None, session_secret=""):
            calls["backtest_session"].append((backtest, session, broker_hostname, session_secret))
            self.ctx = _Ctx(_Live(backtest))
            return self.ctx

        def trading_session(self, *args, **kwargs):
            calls["trading_session"].append((args, kwargs))
            raise AssertionError("trading_session must not be called")

    holder = {}

    def foreverbull_factory(model):
        holder["fb"] = _FB(model)
        return holder["fb"]

    monkeypatch.setattr("foreverbull.broker.backtest.get", get)
    monkeypatch.setattr("foreverbull.broker.backtest.create_session", create_session)
    monkeypatch.setattr("foreverbull_testing.plugin.Foreverbull", foreverbull_factory)

    gen = _backtest(_backtest_request("demo"))
    try:
        result = next(gen)
        assert isinstance(result, Backtest)
        assert result.get_default().name == "demo"
        assert calls["get"] == ["demo"]
        assert calls["create_session"] == ["demo"]
        assert len(calls["backtest_session"]) == 1
        found, session, hostname, secret = calls["backtest_session"][0]
        assert found.name == "demo"
        assert session.port == 27001
        assert session.id == "sess-1"
        assert hostname is None
        assert secret == "port-frame-secret"
        assert calls["trading_session"] == []
        assert holder["fb"].model is not None
        assert os.environ.get("TRADING_ENABLED") == trading_before
    finally:
        gen.close()
    assert holder["fb"].ctx.exited
