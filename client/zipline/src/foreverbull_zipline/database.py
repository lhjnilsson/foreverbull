"""Postgres access for Zipline ingest.

Strategy workers do not use this. They receive bars from the broker.
"""

import logging

from sqlalchemy import create_engine
from sqlalchemy import engine
from sqlalchemy.engine.url import make_url
from sqlalchemy.exc import ArgumentError
from sqlalchemy.exc import DBAPIError

from foreverbull.broker.utils import describe_exception
from foreverbull.exceptions import ConfigurationError
from foreverbull.exceptions import ConnectionError
from foreverbull.exceptions import DatabaseDriverError
from foreverbull.exceptions import DatabaseError


_DEFAULT_DATABASE_DRIVER = "psycopg"
_UNPARSABLE_URL = "<unparsable database URL>"


def _postgres_url(url: str) -> str:
    if url.startswith("postgres://"):
        return url.replace("postgres://", "postgresql://", 1)
    return url


def redact_url(url: str) -> str:
    """The URL with its password shown as ``***``, for logs and error text."""
    try:
        return make_url(url).render_as_string(hide_password=True)
    except ArgumentError:
        return _UNPARSABLE_URL


def _url_password(url: str) -> str | None:
    try:
        return make_url(url).password
    except ArgumentError:
        return None


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
