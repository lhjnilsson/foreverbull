import os
import time

import grpc
import pytest

from testcontainers.core.container import DockerContainer
from testcontainers.core.waiting_utils import wait_container_is_ready
from testcontainers.minio import MinioContainer

from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2_grpc as engine_service_pb2_grpc


@pytest.fixture(scope="function")
def zipline():
    image = os.getenv("BACKTEST_IMAGE")
    if image is None:
        pytest.skip("BACKTEST_IMAGE environment variable is not set")

    container = DockerContainer(image)
    container.with_exposed_ports(50055)
    with container:
        for _ in range(100):
            code, output = container.exec("grpc_health_probe -addr=:50055")
            if code == 0:
                break
            time.sleep(0.1)
        else:
            raise Exception("Failed to start the container")
        yield container


@pytest.fixture(scope="function")
def engine_stub(zipline):
    return engine_service_pb2_grpc.EngineStub(grpc.insecure_channel(f"localhost:{zipline.get_exposed_port(50055)}"))


@pytest.fixture(scope="function")
def storage():
    with MinioContainer("pgsty/silo:RELEASE.2026-09-16T00-00-00Z") as storage:
        wait_container_is_ready(storage, 9000)
        client = storage.get_client()
        client.make_bucket("ingestion")
        yield client
