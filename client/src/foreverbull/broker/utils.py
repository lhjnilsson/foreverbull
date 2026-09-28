import logging
import os

from collections.abc import Iterator
from contextlib import contextmanager

import pynng

from sqlalchemy import create_engine
from sqlalchemy import engine
from sqlalchemy.engine.url import make_url

from foreverbull.broker.config import load_cluster_config


DEFAULT_NNG_HOSTNAME = "127.0.0.1"


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
    parsed = make_url(_postgres_url(url))
    if parsed.host != "postgres":
        return url
    return parsed.set(host="127.0.0.1").render_as_string(hide_password=False)


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
