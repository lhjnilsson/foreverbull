"""Test environment helpers.

`setup_env` fills process env defaults and does not start Docker, create a
network, or require a daemon. Request `fb_env` from CLI, proto, and in-process
socket tests.
"""

import os


def setup_env() -> None:
    """Set test env defaults without creating a Docker network or containers."""
    os.environ.setdefault("BROKER_HOSTNAME", "127.0.0.1")
