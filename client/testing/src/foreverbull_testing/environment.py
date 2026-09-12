"""Test environment helpers.

`setup_env` fills process env defaults and does not start Docker, create a
network, or require a daemon. Request `fb_env` from CLI, proto, and in-process
socket tests.

`local_environment` starts Postgres, Minio, NATS, and a Foreverbull broker
container. Use it only for full-stack tests — not default unit tests. It is not
registered by the pytest plugin.
"""

import os


def setup_env() -> None:
    """Set test env defaults without creating a Docker network or containers."""
    os.environ.setdefault("BROKER_HOSTNAME", "127.0.0.1")


def local_environment():
    """Full-stack Docker environment. Not registered by the pytest plugin."""
    from testcontainers.core.container import DockerContainer
    from testcontainers.core.network import Network
    from testcontainers.core.waiting_utils import wait_for_logs
    from testcontainers.minio import MinioContainer
    from testcontainers.nats import NatsContainer
    from testcontainers.postgres import PostgresContainer

    with Network() as network:
        postgres = PostgresContainer(
            "postgres:13.3-alpine",
            network=network.name,
            hostname="postgres",
            username="postgres",
            password="postgres",
            dbname="postgres",
        )
        minio = MinioContainer("minio/minio:latest", network=network.name, hostname="minio")
        nats = NatsContainer("nats:2.10-alpine", network=network.name, hostname="nats")
        nats = nats.with_command("-js")
        with postgres as postgres, minio as minio, nats as nats:
            foreverbull = DockerContainer(os.environ.get("BROKER_IMAGE", ""))
            foreverbull.with_network(network)
            foreverbull.with_env("POSTGRES_URL", "postgres://postgres:postgres@postgres:5432/postgres")
            foreverbull.with_env("NATS_URL", "nats://nats:4222")
            foreverbull.with_env("MINIO_URL", "minio:9000")
            foreverbull.with_env(
                "BACKTEST_IMAGE",
                os.environ.get("BACKTEST_IMAGE", "lhjnilsson/zipline:latest"),
            )
            foreverbull.with_volume_mapping("/var/run/docker.sock", "/var/run/docker.sock", mode="rw")
            with foreverbull as foreverbull:
                wait_for_logs(foreverbull, "RUNNING", 10)
                yield foreverbull
