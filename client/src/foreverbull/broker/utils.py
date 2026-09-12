import logging
import os
import re

from contextlib import contextmanager
from typing import Iterator

import pynng

from sqlalchemy import create_engine
from sqlalchemy import engine


def _get_postgres_urls(base_url: str) -> Iterator[str]:
    if base_url.startswith("postgres://"):
        base_url = base_url.replace("postgres://", "postgresql://", 1)

    yield base_url

    database_host = re.search(r"@([^/]+):", base_url)
    if database_host is None:
        raise Exception("Could not find database host in URL")
    database_host = database_host.group(1)
    database_port = re.search(r":(\d+)/", base_url)
    if database_port is None:
        raise Exception("Could not find database port in URL")
    database_port = database_port.group(1)

    new_url = ""
    for hostname in ["localhost", "postgres", "127.0.0.1"]:
        new_url = base_url.replace(f"@{database_host}:", f"@{hostname}:", 1)
        yield new_url.replace(f":{database_port}", ":5432", 1)


# Hacky way to get the database URL, TODO: find a better way, ie pass correct url
def get_engine(url: str) -> engine.Engine:
    log = logging.getLogger().getChild(__name__)
    for url in _get_postgres_urls(url):
        try:
            engine = create_engine(url)
            engine.connect()
            return engine
        except Exception:
            log.warning(f"Could not connect to {url}")
    raise Exception("Could not connect to database")


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
