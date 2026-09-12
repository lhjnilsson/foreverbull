import logging
import os

from contextlib import contextmanager
from typing import Iterator

import pynng

from sqlalchemy import create_engine
from sqlalchemy import engine


def _get_postgres_urls(base_url: str) -> Iterator[str]:
    if base_url.startswith("postgres://"):
        base_url = base_url.replace("postgres://", "postgresql://", 1)
    yield base_url


def get_engine(url: str) -> engine.Engine:
    log = logging.getLogger().getChild(__name__)
    url = next(_get_postgres_urls(url))
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
    hostname = os.environ.get("BROKER_HOSTNAME", "127.0.0.1")
    port = os.environ.get("NAMESPACE_PORT", None)
    if port is None:
        raise Exception("Namespace port not set")
    socket = pynng.Req0(dial=f"tcp://{hostname}:{port}", block_on_dial=True)
    socket.recv_timeout = 5000
    socket.send_timeout = 5000
    try:
        yield socket
    finally:
        socket.close()
