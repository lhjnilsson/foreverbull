import os

import pytest
from testcontainers.core.container import DockerContainer
from testcontainers.core.network import Network
from testcontainers.core.waiting_utils import wait_for_logs
from testcontainers.minio import MinioContainer
from testcontainers.nats import NatsContainer
from testcontainers.postgres import PostgresContainer


@pytest.fixture(scope="session")
def network():
    with Network() as network:
        yield network


@pytest.fixture(scope="session")
def postgres(network):
    postgres = PostgresContainer(
        "postgres:13.3-alpine",
        network=network.name,
        hostname="postgres",
        username="postgres",
        password="postgres",
        dbname="postgres",
    )
    with postgres as postgres:
        yield postgres


@pytest.fixture(scope="session")
def minio(network):
    minio = MinioContainer("minio/minio:latest", network=network.name, hostname="minio")
    with minio as minio:
        yield minio


@pytest.fixture(scope="session")
def nats(network):
    nats = NatsContainer("nats:2.10-alpine", network=network.name, hostname="nats")
    nats = nats.with_command("-js")
    with nats as nats:
        yield nats


@pytest.fixture(scope="session")
def foreverbull_container(network, minio, nats, postgres):
    foreverbull = DockerContainer(os.environ.get("BROKER_IMAGE", ""))
    foreverbull.with_network(network)
    foreverbull.with_env("POSTGRES_URL", "postgres://postgres:postgres@postgres:5432/postgres")
    foreverbull.with_env("NATS_URL", "nats://nats:4222")
    foreverbull.with_env("MINIO_URL", "minio:9000")
    foreverbull.with_env("BACKTEST_IMAGE", os.environ.get("BACKTEST_IMAGE", "lhjnilsson/zipline:latest"))
    foreverbull.with_volume_mapping("/var/run/docker.sock", "/var/run/docker.sock", mode="rw")
    with foreverbull as foreverbull:
        wait_for_logs(foreverbull, "RUNNING", 10)
        yield foreverbull
