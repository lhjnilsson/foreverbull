import logging
import os
import re

from collections.abc import Iterator
from contextlib import contextmanager
from urllib.parse import quote_plus

import pynng

from sqlalchemy import create_engine
from sqlalchemy import engine
from sqlalchemy.engine.url import make_url
from sqlalchemy.exc import ArgumentError
from sqlalchemy.exc import DBAPIError

from foreverbull.broker.config import load_cluster_config
from foreverbull.exceptions import ConfigurationError
from foreverbull.exceptions import ConnectionError
from foreverbull.exceptions import DatabaseDriverError
from foreverbull.exceptions import DatabaseError


DEFAULT_NNG_HOSTNAME = "127.0.0.1"
_DEFAULT_DATABASE_DRIVER = "psycopg"
_UNPARSABLE_URL = "<unparsable database URL>"
# The userinfo password of any scheme://user:password@ URL in free text.
_URL_PASSWORD = re.compile(r"(\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]*:)[^\s@]*@")


def nng_hostname() -> str:
    return os.environ.get("BROKER_HOSTNAME", DEFAULT_NNG_HOSTNAME)


def _postgres_url(url: str) -> str:
    if url.startswith("postgres://"):
        return url.replace("postgres://", "postgresql://", 1)
    return url


def _get_postgres_urls(base_url: str) -> Iterator[str]:
    yield _postgres_url(base_url)


def database_url_for_host(url: str) -> str:
    """Use the Kind host port when a local worker is given the in-cluster Service host.

    The broker runs in the cluster and publishes ``postgres://...@postgres:5432/...``.
    ``fbull backtest run`` executes the algorithm on the host, where that name does not
    resolve. Local Kind maps Postgres to ``127.0.0.1:5432``. In-cluster workers set
    ``BROKER_HOSTNAME`` and keep the Service name. Remote clusters keep the URL the
    broker sent.
    """
    if not url or nng_hostname() != DEFAULT_NNG_HOSTNAME:
        return url
    if load_cluster_config().get("cluster") != "local":
        return url
    try:
        parsed = make_url(_postgres_url(url))
    except ArgumentError:
        return url
    if parsed.host != "postgres":
        return url
    return parsed.set(host="127.0.0.1").render_as_string(hide_password=False)


def redact_url(url: str) -> str:
    """The URL with its password shown as ``***``, for logs and error text."""
    try:
        return make_url(url).render_as_string(hide_password=True)
    except ArgumentError:
        return _UNPARSABLE_URL


def redact_text(text: str, password: str | None = None) -> str:
    """Text with URL passwords, and the given password anywhere, shown as ``***``.

    A driver or SQLAlchemy message can quote the URL or the password itself,
    so exception text is passed through this before it is logged or raised.
    """
    redacted = _URL_PASSWORD.sub(r"\1***@", text)
    if password:
        redacted = redacted.replace(quote_plus(password), "***").replace(password, "***")
    return redacted


def _url_password(url: str) -> str | None:
    try:
        return make_url(url).password
    except ArgumentError:
        return None


def describe_exception(exc: BaseException, password: str | None = None) -> str:
    """``Type: message`` with passwords redacted, for a --verbose cause line."""
    text = f"{type(exc).__name__}: {exc}" if str(exc) else type(exc).__name__
    return redact_text(text, password)


def _driver_error(exc: ImportError, shown: str, password: str | None) -> DatabaseDriverError:
    module = getattr(exc, "name", None) or _DEFAULT_DATABASE_DRIVER
    driver = module.split(".")[0]
    cause = describe_exception(exc, password)
    logging.getLogger().getChild(__name__).error("Database driver %s is not installed for %s: %s", driver, shown, cause)
    return DatabaseDriverError(f"Database driver {driver} is not installed", driver=driver, cause=cause)


def get_engine(url: str) -> engine.Engine:
    """Open and test an engine. Raised errors and log lines never carry the password.

    A missing driver, a bad URL and a failed connection are separate errors, so
    a missing driver is not blamed on the database. The original exception is
    not chained: its text or traceback can quote the URL. Its redacted
    ``Type: message`` is kept on the new error as ``cause``.
    """
    log = logging.getLogger().getChild(__name__)
    url = _postgres_url(url)
    shown = redact_url(url)
    password = _url_password(url)
    try:
        db_engine = create_engine(url)
        with db_engine.connect():
            pass
    except ImportError as exc:
        raise _driver_error(exc, shown, password) from None
    except ArgumentError:
        log.error("Invalid database URL %s", shown)
        raise ConfigurationError(f"Invalid database URL {shown}") from None
    except (DBAPIError, OSError) as exc:
        cause = describe_exception(exc, password)
        log.error("Could not connect to database at %s: %s", shown, cause)
        raise ConnectionError(f"Could not connect to database at {shown}", cause=cause) from None
    except Exception as exc:  # noqa: BLE001 - re-raised redacted; chaining it could print the password
        cause = describe_exception(exc, password)
        log.error("Could not open database at %s: %s", shown, cause)
        raise DatabaseError(f"Could not open database at {shown}", cause=cause) from None
    return db_engine


@contextmanager
def namespace_socket() -> Iterator[pynng.Socket]:
    port = os.environ.get("NAMESPACE_PORT", None)
    if port is None:
        raise ConfigurationError("Namespace port not set")
    socket = pynng.Req0(dial=f"tcp://{nng_hostname()}:{port}", block_on_dial=True)
    socket.recv_timeout = 5000
    socket.send_timeout = 5000
    try:
        yield socket
    finally:
        socket.close()
