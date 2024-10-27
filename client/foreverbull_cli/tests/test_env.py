import traceback

from collections import namedtuple
from datetime import datetime
from unittest.mock import PropertyMock
from unittest.mock import patch

import docker.errors
import pytest

from typer.testing import CliRunner

from foreverbull.pb.foreverbull.backtest import ingestion_pb2
from foreverbull_cli.env import env


runner = CliRunner(mix_stderr=False)

container = namedtuple("container", ["id", "name", "image", "status", "health"])
image = namedtuple("image", ["id", "short_id", "tags", "created"])


class MockedDockerProperty:
    def __init__(self, resources: dict, on_not_found: Exception = docker.errors.NotFound("")):
        self.resources = resources
        self.on_not_found = on_not_found

    def get(self, key):
        if key in self.resources:
            return self.resources[key]
        else:
            raise self.on_not_found

    def create(self, *args, **kwargs):
        pass

    def pull(self, image: str):
        pass

    def run(self, image: str, **kwargs):
        self.resources[kwargs["name"]] = container(
            id=kwargs["name"],
            name=kwargs["name"],
            image=image,
            status="running",
            health="healthy",
        )


@pytest.mark.parametrize(
    "containers, images",
    [
        ({}, {}),
        (
            {
                "foreverbull_postgres": container(
                    id="container1",
                    name="container1",
                    image="image1",
                    status="running",
                    health="healthy",
                ),
                "foreverbull_nats": container(
                    id="container2",
                    name="container2",
                    image="image2",
                    status="running",
                    health="healthy",
                ),
                "foreverbull_minio": container(
                    id="container3",
                    name="container3",
                    image="image3",
                    status="running",
                    health="healthy",
                ),
                "foreverbull_foreverbull": container(
                    id="container4",
                    name="container4",
                    image="image4",
                    status="running",
                    health="healthy",
                ),
            },
            {
                "postgres:13.3-alpine": image(
                    id="image1",
                    short_id="image1",
                    tags=["image1"],
                    created=datetime(2021, 1, 1, 0, 0, 0),
                ),
                "nats:2.10-alpine": image(
                    id="image2",
                    short_id="image2",
                    tags=["image2"],
                    created=datetime(2021, 1, 1, 0, 0, 0),
                ),
                "minio/minio:latest": image(
                    id="image3",
                    short_id="image3",
                    tags=["image3"],
                    created=datetime(2021, 1, 1, 0, 0, 0),
                ),
                "lhjnilsson/foreverbull:latest": image(
                    id="image4",
                    short_id="image4",
                    tags=["image4"],
                    created=datetime(2021, 1, 1, 0, 0, 0),
                ),
            },
        ),
    ],
)
def test_env_status(containers, images):
    with (
        patch("docker.client.DockerClient.containers", new_callable=PropertyMock) as mock_containers,
        patch("docker.client.DockerClient.images", new_callable=PropertyMock) as mock_images,
    ):
        mock_containers.return_value = MockedDockerProperty(containers)
        mock_images.return_value = MockedDockerProperty(images, on_not_found=docker.errors.ImageNotFound(""))
        result = runner.invoke(env, ["status"])

        if result.exception and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert result.exit_code == 0


def test_env_start():
    with (
        patch("docker.client.DockerClient.containers", new_callable=PropertyMock) as mock_containers,
        patch("docker.client.DockerClient.images", new_callable=PropertyMock) as mock_images,
        patch("docker.client.DockerClient.networks", new_callable=PropertyMock) as mock_network,
        patch("foreverbull.broker.backtest.ingest") as mock_ingest,
        patch("foreverbull.broker.backtest.get_ingestion") as mock_get_ingestion,
    ):
        mock_containers.return_value = MockedDockerProperty({})
        mock_images.return_value = MockedDockerProperty({}, on_not_found=docker.errors.ImageNotFound(""))
        mock_network.return_value = MockedDockerProperty({})
        mock_ingest.return_value = None
        mock_get_ingestion.return_value = (None, ingestion_pb2.IngestionStatus.READY)
        result = runner.invoke(env, ["start"])

        if result.exception and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert result.exit_code == 0


def test_env_stop():
    with (
        patch("docker.client.DockerClient.containers", new_callable=PropertyMock) as mock_containers,
        patch("docker.client.DockerClient.images", new_callable=PropertyMock) as mock_images,
        patch("docker.client.DockerClient.networks", new_callable=PropertyMock) as mock_network,
    ):
        mock_containers.return_value = MockedDockerProperty({})
        mock_images.return_value = MockedDockerProperty({}, on_not_found=docker.errors.ImageNotFound(""))
        mock_network.return_value = MockedDockerProperty({})
        result = runner.invoke(env, ["stop"])

        if result.exception and result.exc_info:
            traceback.print_exception(*result.exc_info)
        assert result.exit_code == 0
