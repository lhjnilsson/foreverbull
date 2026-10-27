import os
import re

from collections.abc import Iterator
from contextlib import contextmanager
from urllib.parse import quote_plus

import pynng

from foreverbull.exceptions import ConfigurationError


DEFAULT_NNG_HOSTNAME = "127.0.0.1"
# The userinfo password of any scheme://user:password@ URL in free text.
_URL_PASSWORD = re.compile(r"(\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]*:)[^\s@]*@")


def nng_hostname() -> str:
    return os.environ.get("BROKER_HOSTNAME", DEFAULT_NNG_HOSTNAME)


def redact_text(text: str, password: str | None = None) -> str:
    """Text with URL passwords, and the given password anywhere, shown as ``***``.

    A driver or SQLAlchemy message can quote the URL or the password itself,
    so exception text is passed through this before it is logged or raised.
    """
    redacted = _URL_PASSWORD.sub(r"\1***@", text)
    if password:
        redacted = redacted.replace(quote_plus(password), "***").replace(password, "***")
    return redacted


def describe_exception(exc: BaseException, password: str | None = None) -> str:
    """``Type: message`` with passwords redacted, for a --verbose cause line."""
    text = f"{type(exc).__name__}: {exc}" if str(exc) else type(exc).__name__
    return redact_text(text, password)


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
