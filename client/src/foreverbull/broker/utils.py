import logging
import os

from collections.abc import Iterator
from contextlib import contextmanager

import pynng

from sqlalchemy import create_engine
from sqlalchemy import engine


DEFAULT_NNG_HOSTNAME = "127.0.0.1"


def nng_hostname() -> str:
    return os.environ.get("BROKER_HOSTNAME", DEFAULT_NNG_HOSTNAME)


def _postgres_url(url: str) -> str:
    if url.startswith("postgres://"):
        return url.replace("postgres://", "postgresql://", 1)
    return url


def _get_postgres_urls(base_url: str) -> Iterator[str]:
    yield _postgres_url(base_url)


def get_engine(url: str) -> engine.Engine:
    log = logging.getLogger().getChild(__name__)
    url = _postgres_url(url)
    try:
        db_engine = create_engine(url)
        with db_engine.connect():
            pass
        return db_engine
    except Exception as exc:
        log.error("Could not connect to database at %s", url)
        raise Exception(f"Could not connect to database at {url}") from exc


@contextmanager
def namespace_socket() -> Iterator[pynng.Socket]:
    port = os.environ.get("NAMESPACE_PORT", None)
    if port is None:
        raise Exception("Namespace port not set")
    socket = pynng.Req0(dial=f"tcp://{nng_hostname()}:{port}", block_on_dial=True)
    socket.recv_timeout = 5000
    socket.send_timeout = 5000
    try:
        yield socket
    finally:
        socket.close()
