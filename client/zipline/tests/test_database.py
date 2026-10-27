"""get_engine reports the real cause and never shows the database password."""

import logging
import sys
import traceback

import pytest

from foreverbull_zipline.database import get_engine
from foreverbull_zipline.database import redact_url

from foreverbull.exceptions import ConfigurationError
from foreverbull.exceptions import ConnectionError
from foreverbull.exceptions import DatabaseDriverError
from foreverbull.exceptions import DatabaseError


SECRET = "n0t-the-default-Pa55"
REDACTED_URL = "postgresql://foreverbull:***@127.0.0.1:1/foreverbull"


def _everything_shown(exc: BaseException, caplog: pytest.LogCaptureFixture) -> str:
    """The exception as a traceback would print it, plus every log line."""
    return "".join(traceback.format_exception(exc)) + caplog.text


def test_redact_url_hides_only_the_password():
    assert redact_url(f"postgresql://foreverbull:{SECRET}@postgres:5432/foreverbull") == (
        "postgresql://foreverbull:***@postgres:5432/foreverbull"
    )
    assert redact_url("not a url") == "<unparsable database URL>"


def test_get_engine_uses_provided_url_only(monkeypatch):
    attempted = []

    def fake_create_engine(url):
        attempted.append(url)
        raise OSError("refused")

    monkeypatch.setattr("foreverbull_zipline.database.create_engine", fake_create_engine)
    with pytest.raises(
        ConnectionError,
        match=r"Could not connect to database at postgresql://user:\*\*\*@db.internal:5656/demo",
    ):
        get_engine("postgres://user:password@db.internal:5656/demo")
    assert attempted == ["postgresql://user:password@db.internal:5656/demo"]


def test_get_engine_missing_driver_names_the_driver(monkeypatch, caplog):
    # sys.modules[...] = None makes the import fail the way a missing package does.
    monkeypatch.setitem(sys.modules, "psycopg", None)
    caplog.set_level(logging.DEBUG)

    with pytest.raises(DatabaseDriverError) as raised:
        get_engine(f"postgresql+psycopg://foreverbull:{SECRET}@127.0.0.1:1/foreverbull")

    assert raised.value.driver == "psycopg"
    assert str(raised.value) == "Database driver psycopg is not installed"
    assert raised.value.cause is not None
    assert raised.value.cause.startswith("ModuleNotFoundError:")
    assert "psycopg" in raised.value.cause
    assert raised.value.__cause__ is None
    shown = _everything_shown(raised.value, caplog)
    assert "Could not connect" not in shown
    assert SECRET not in shown
    assert "postgresql+psycopg://foreverbull:***@127.0.0.1:1/foreverbull" in caplog.text


def test_get_engine_module_not_found_from_create_engine(monkeypatch, caplog):
    def missing_driver(url):
        raise ModuleNotFoundError("No module named 'psycopg'", name="psycopg")

    monkeypatch.setattr("foreverbull_zipline.database.create_engine", missing_driver)
    with pytest.raises(DatabaseDriverError) as raised:
        get_engine(f"postgres://foreverbull:{SECRET}@127.0.0.1:1/foreverbull")

    assert raised.value.cause == "ModuleNotFoundError: No module named 'psycopg'"
    assert SECRET not in _everything_shown(raised.value, caplog)
    assert REDACTED_URL in caplog.text


def test_get_engine_connection_failure_is_a_connection_error(caplog):
    pytest.importorskip("psycopg")
    caplog.set_level(logging.DEBUG)

    # Port 1 on loopback refuses at once, so this is a real connection failure.
    with pytest.raises(ConnectionError) as raised:
        get_engine(f"postgresql+psycopg://foreverbull:{SECRET}@127.0.0.1:1/foreverbull")

    assert (
        str(raised.value)
        == "Could not connect to database at postgresql+psycopg://foreverbull:***@127.0.0.1:1/foreverbull"
    )
    assert raised.value.cause is not None
    assert raised.value.cause.startswith("OperationalError:")
    assert raised.value.__cause__ is None
    shown = _everything_shown(raised.value, caplog)
    assert SECRET not in shown
    assert "***" in shown


def test_get_engine_driver_quoting_the_password_is_redacted(monkeypatch, caplog):
    def leaky(url):
        raise OSError(f"refused for {url} with password {SECRET}")

    monkeypatch.setattr("foreverbull_zipline.database.create_engine", leaky)
    with pytest.raises(ConnectionError) as raised:
        get_engine(f"postgres://foreverbull:{SECRET}@127.0.0.1:1/foreverbull")

    assert str(raised.value) == f"Could not connect to database at {REDACTED_URL}"
    assert raised.value.cause == f"OSError: refused for {REDACTED_URL} with password ***"
    assert SECRET not in _everything_shown(raised.value, caplog)


def test_get_engine_other_failure_is_not_blamed_on_the_connection(monkeypatch, caplog):
    def broken(url):
        raise AttributeError("module has no attribute 'paramstyle'")

    monkeypatch.setattr("foreverbull_zipline.database.create_engine", broken)
    with pytest.raises(DatabaseError) as raised:
        get_engine(f"postgres://foreverbull:{SECRET}@127.0.0.1:1/foreverbull")

    assert not isinstance(raised.value, ConnectionError)
    assert str(raised.value) == f"Could not open database at {REDACTED_URL}"
    assert raised.value.cause == "AttributeError: module has no attribute 'paramstyle'"
    assert SECRET not in _everything_shown(raised.value, caplog)


def test_get_engine_bad_url_is_a_configuration_error(caplog):
    with pytest.raises(ConfigurationError) as raised:
        get_engine(f"no-scheme-{SECRET}")

    assert str(raised.value) == "Invalid database URL <unparsable database URL>"
    assert SECRET not in _everything_shown(raised.value, caplog)
