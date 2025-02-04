from pathlib import Path
from unittest.mock import MagicMock
from unittest.mock import patch

import docker.client
import docker.errors
import pytest

from docker.models.containers import Container
from docker.models.images import Image

from foreverbull_cli.env.environment import BaseContainer
from foreverbull_cli.env.environment import Config
from foreverbull_cli.env.environment import ContainerStatus
from foreverbull_cli.env.environment import Environment
from foreverbull_cli.env.environment import ForeverbullContainer
from foreverbull_cli.env.environment import GrafanaContainer
from foreverbull_cli.env.environment import MinioContainer
from foreverbull_cli.env.environment import NatsContainer
from foreverbull_cli.env.environment import PostgresContainer


class TestConfig:
    @pytest.fixture
    def uut(self):
        return Config()

    def test_version(self, uut: Config):
        assert uut.version == "latest"

    def test_network_name(self, uut: Config):
        assert uut.network_name == "foreverbull"

    def test_postgres_image(self, uut: Config):
        assert uut.postgres_image == "postgres:13.3-alpine"

    def test_minio_image(self, uut: Config):
        assert uut.minio_image == "minio/minio:latest"

    def test_nats_image(self, uut: Config):
        assert uut.nats_image == "nats:2.10-alpine"

    def test_foreverbull_image(self, uut: Config):
        assert uut.foreverbull_image == "lhjnilsson/foreverbull:latest"

    def test_backtest_image(self, uut: Config):
        assert uut.backtest_image == "lhjnilsson/zipline:latest"

    def test_grafana_image(self, uut: Config):
        assert uut.grafana_image == "lhjnilsson/fb-grafana:latest"


class TestEnvironment:
    @pytest.fixture
    def uut(self):
        return Environment()

    def test_postgres_location(self, uut: Environment):
        assert uut.postgres_location == Path(".foreverbull/postgres")

    def test_minio_location(self, uut: Environment):
        assert uut.minio_location == Path(".foreverbull/minio")

    def test_nats_location(self, uut: Environment):
        assert uut.nats_location == Path(".foreverbull/nats")


class DockerMocks:
    @pytest.fixture
    def mocked_container(self):
        with patch("docker.client.DockerClient.containers", new_callable=MagicMock) as mock:
            yield mock

    @pytest.fixture
    def mocked_images(self):
        with patch("docker.client.DockerClient.images", new_callable=MagicMock) as mock:
            yield mock


class TestBaseContainer(DockerMocks):
    @pytest.fixture
    def uut(self):
        config = Config()
        env = Environment()
        return BaseContainer(config, env)

    @pytest.mark.parametrize(
        "status, expected_status",
        [
            ("running", ContainerStatus.RUNNING),
            ("exited", ContainerStatus.STOPPED),
            ("created", ContainerStatus.STOPPED),
            ("undefined", ContainerStatus.NOT_FOUND),
        ],
    )
    def test_status(self, uut: BaseContainer, mocked_container: MagicMock, status, expected_status):
        uut.name = "test_container"

        mocked_container.get.return_value = Container(attrs={"State": {"Status": status}})
        assert uut.status() == expected_status
        mocked_container.get.assert_called_with("test_container")

    def test_container_id(self, uut: BaseContainer, mocked_container: MagicMock):
        uut.name = "test_container"
        mocked_container.get.return_value = Container(attrs={"Id": "test_id"})
        assert uut.container_id() == "test_id"
        mocked_container.get.assert_called_with("test_container")

    def test_image_version(self, uut: BaseContainer, mocked_container: MagicMock, mocked_images: MagicMock):
        uut.name = "test_container"
        client = MagicMock()
        client.images = mocked_images
        mocked_container.get.return_value = Container(attrs={"Image": "test_image:latest"}, client=client)
        mocked_images.get.return_value = Image(attrs={"RepoTags": ["test_image:latest"]})
        assert uut.image_version() == "test_image:latest"
        mocked_container.get.assert_called_with("test_container")
        mocked_images.get.assert_called_with("latest")

    def test_get_or_download_image_found(self, uut: BaseContainer, mocked_images: MagicMock):
        uut.image = "test_image:latest"
        client = MagicMock()
        client.images = mocked_images
        mocked_images.get.return_value = Image(attrs={"RepoTags": ["test_image:latest"]})
        uut.get_or_download_image()
        mocked_images.get.assert_called_with("test_image:latest")
        assert not mocked_images.pull.called

    def test_get_or_download_image_not_found(self, uut: BaseContainer, mocked_images: MagicMock):
        uut.image = "test_image:latest"
        client = MagicMock()
        client.images = mocked_images
        mocked_images.get.side_effect = docker.errors.ImageNotFound("test_image:latest")
        uut.get_or_download_image()
        mocked_images.get.assert_called_with("test_image:latest")
        mocked_images.pull.assert_called_with("test_image:latest")

    def test_create(self, uut: BaseContainer, mocked_container: MagicMock):
        uut.image = "test_image:latest"
        uut.name = "test_container"
        uut.create()
        mocked_container.create.assert_called_with(
            image="test_image:latest", name="test_container", detach=True, network="foreverbull", hostname=""
        )

    def test_start(self, uut: BaseContainer, mocked_container: MagicMock):
        uut.name = "test_container"
        uut.start()
        mocked_container.get.assert_called_with("test_container")
        mocked_container.get.return_value.start.assert_called()

    def test_stop(self, uut: BaseContainer, mocked_container: MagicMock):
        uut.name = "test_container"
        uut.stop()
        mocked_container.get.assert_called_with("test_container")
        mocked_container.get.return_value.stop.assert_called()

    def test_remove(self, uut: BaseContainer, mocked_container: MagicMock):
        uut.name = "test_container"
        uut.remove()
        mocked_container.get.assert_called_with("test_container")
        mocked_container.get.return_value.remove.assert_called()

    def test_update(self, uut: BaseContainer):
        # TODO
        pass


class TestPostgresContainer(DockerMocks):
    @pytest.fixture
    def uut(self):
        config = Config()
        env = Environment()
        return PostgresContainer(config, env)

    def test_create(self, uut: PostgresContainer, mocked_container: MagicMock):
        uut.create()
        assert mocked_container.create.called


class TestMinioContainer(DockerMocks):
    @pytest.fixture
    def uut(self):
        config = Config()
        env = Environment()
        return MinioContainer(config, env)

    def test_create(self, uut: MinioContainer, mocked_container: MagicMock):
        uut.create()
        assert mocked_container.create.called


class TestNatsContainer(DockerMocks):
    @pytest.fixture
    def uut(self):
        config = Config()
        env = Environment()
        return NatsContainer(config, env)

    def test_create(self, uut: NatsContainer, mocked_container: MagicMock):
        uut.create()
        assert mocked_container.create.called


class TestForeverbullContainer(DockerMocks):
    @pytest.fixture
    def uut(self):
        config = Config()
        env = Environment()
        return ForeverbullContainer(config, env)

    def test_create(self, uut: ForeverbullContainer, mocked_container: MagicMock):
        uut.create()
        assert mocked_container.create.called


class TestGrafanaContainer(DockerMocks):
    @pytest.fixture
    def uut(self):
        config = Config()
        env = Environment()
        return GrafanaContainer(config, env)

    def create(self, uut: GrafanaContainer, mocked_container: MagicMock):
        uut.create()
        assert mocked_container.create.called
