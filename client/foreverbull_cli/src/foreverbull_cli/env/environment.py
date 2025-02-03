import os

from enum import StrEnum
from pathlib import Path

import docker
import docker.errors
import docker.models.containers


_docker = docker.from_env()


class ContainerStatus(StrEnum):
    NOT_FOUND = "not_found"
    RUNNING = "running"
    STOPPED = "stopped"

    @classmethod
    def from_container(cls, container: docker.models.containers.Container):
        if container.status == "running":
            return cls.RUNNING
        elif container.status == "exited" or container.status == "created":
            return cls.STOPPED
        else:
            return cls.NOT_FOUND


class Config:
    @property
    def version(self) -> str:
        return os.environ.get("VERSION", "latest")

    @property
    def network_name(self) -> str:
        return "foreverbull"

    @property
    def postgres_image(self) -> str:
        return "postgres:13.3-alpine"

    @property
    def minio_image(self) -> str:
        return "minio/minio:latest"

    @property
    def nats_image(self) -> str:
        return "nats:2.10-alpine"

    @property
    def foreverbull_image(self) -> str:
        return f"lhjnilsson/foreverbull:{self.version}"

    @property
    def backtest_image(self) -> str:
        return f"lhjnilsson/zipline:{self.version}"

    @property
    def grafana_image(self) -> str:
        return f"lhjnilsson/fb-grafana:{self.version}"


class Environment:
    def __init__(self, path: str | None = None):
        if path is None:
            self.path = Path(".") / ".foreverbull"
        else:
            self.path = Path(path) / ".foreverbull"
        if not self.path.exists():
            self.path.mkdir(parents=True)
        for loc in [
            self.postgres_location,
            self.minio_location,
            self.nats_location,
        ]:
            if not loc.exists():
                loc.mkdir()

    @property
    def postgres_location(self) -> Path:
        return self.path / "postgres"

    @property
    def minio_location(self) -> Path:
        return self.path / "minio"

    @property
    def nats_location(self) -> Path:
        return self.path / "nats"


class BaseContainer:
    def __init__(self, config: Config, environment: Environment):
        self.image = ""
        self.name = ""
        self.hostname = ""
        self.config = config
        self.environment = environment

    def status(self) -> ContainerStatus:
        try:
            container = _docker.containers.get(self.name)
        except docker.errors.NotFound:
            return ContainerStatus.NOT_FOUND

        return ContainerStatus.from_container(container)

    def container_id(self) -> str:
        try:
            container = _docker.containers.get(self.name)
        except docker.errors.NotFound:
            return "not found"
        return container.short_id

    def image_version(self) -> str:
        try:
            container = _docker.containers.get(self.name)
        except docker.errors.NotFound:
            return "not found"
        if container.image is None or len(container.image.tags) == 0:
            return "not known"
        return container.image.tags[0]

    def get_or_download_image(self) -> None:
        try:
            _docker.images.get(self.image)
        except docker.errors.ImageNotFound:
            _docker.images.pull(self.image)

    def create(self, *args, **kwargs) -> ContainerStatus:
        container = _docker.containers.create(
            image=self.image,
            name=self.name,
            detach=True,
            network=self.config.network_name,
            hostname=self.hostname,
            **kwargs,
        )
        return ContainerStatus.from_container(container)

    def start(self) -> ContainerStatus:
        try:
            container = _docker.containers.get(self.name)
        except docker.errors.NotFound:
            return ContainerStatus.NOT_FOUND
        container.start()
        return ContainerStatus.from_container(container)

    def stop(self, remove: bool = False) -> ContainerStatus:
        try:
            container = _docker.containers.get(self.name)
        except docker.errors.NotFound:
            return ContainerStatus.NOT_FOUND
        container.stop()
        container.reload()
        return ContainerStatus.from_container(container)

    def remove(self) -> ContainerStatus:
        try:
            container = _docker.containers.get(self.name)
        except docker.errors.NotFound:
            return ContainerStatus.NOT_FOUND
        container.remove()
        return ContainerStatus.NOT_FOUND

    def update(self) -> ContainerStatus:
        return ContainerStatus.NOT_FOUND


class PostgresContainer(BaseContainer):
    def __init__(self, config: Config, environment: Environment):
        super().__init__(config, environment)
        self.name = "foreverbull_postgres"
        self.hostname = "postgres"
        self.image = config.postgres_image

    def create(self, *args, **kwargs) -> ContainerStatus:
        kwargs["ports"] = {"5432/tcp": 5432}
        kwargs["environment"] = {
            "POSTGRES_USER": "foreverbull",
            "POSTGRES_PASSWORD": "foreverbull",
            "POSTGRES_DB": "foreverbull",
            "PGDATA": "/pgdata",
        }
        kwargs["healthcheck"] = {
            "test": ["CMD", "pg_isready", "-U", "foreverbull"],
            "interval": 10000000000,
            "timeout": 5000000000,
            "retries": 5,
        }
        kwargs["volumes"] = {
            str((self.environment.postgres_location / "data").absolute()): {
                "bind": "/pgdata",
                "mode": "rw",
            },
        }
        return super().create(*args, **kwargs)


class MinioContainer(BaseContainer):
    def __init__(self, config: Config, environment: Environment):
        super().__init__(config, environment)
        self.name = "foreverbull_minio"
        self.hostname = "minio"
        self.image = config.minio_image

    def create(self, *args, **kwargs) -> ContainerStatus:
        kwargs["ports"] = {"9000/tcp": 9000}
        kwargs["volumes"] = {
            str((self.environment.minio_location / "data").absolute()): {
                "bind": "/data",
                "mode": "rw",
            },
        }
        kwargs["command"] = 'server --console-address ":9001" /data'
        return super().create(*args, **kwargs)


class NatsContainer(BaseContainer):
    def __init__(self, config: Config, environment: Environment):
        super().__init__(config, environment)
        self.name = "foreverbull_nats"
        self.hostname = "nats"
        self.image = config.nats_image

    def create(self, *args, **kwargs) -> ContainerStatus:
        kwargs["ports"] = {"4222/tcp": 4222}
        kwargs["healthcheck"] = {
            "test": ["CMD", "nats-server", "-sl"],
            "interval": 10000000000,
            "timeout": 5000000000,
            "retries": 5,
        }
        kwargs["volumes"] = {
            str((self.environment.nats_location / "data").absolute()): {
                "bind": "/data",
                "mode": "rw",
            },
        }
        kwargs["command"] = "-js -sd /var/lib/nats/data"
        return super().create(*args, **kwargs)


class ForeverbullContainer(BaseContainer):
    def __init__(self, config: Config, environment: Environment):
        super().__init__(config, environment)
        self.name = "foreverbull_foreverbull"
        self.hostname = "foreverbull"
        self.image = config.foreverbull_image

    def create(self, *args, **kwargs) -> ContainerStatus:
        kwargs["ports"] = {
            "50055/tcp": 50055,
            "27000/tcp": 27000,
            "27001/tcp": 27001,
            "27002/tcp": 27002,
            "27003/tcp": 27003,
            "27004/tcp": 27004,
            "27005/tcp": 27005,
            "27006/tcp": 27006,
            "27007/tcp": 27007,
            "27008/tcp": 27008,
            "27009/tcp": 27009,
            "27010/tcp": 27010,
            "27011/tcp": 27011,
            "27012/tcp": 27012,
            "27013/tcp": 27013,
            "27014/tcp": 27014,
            "27015/tcp": 27015,
        }
        kwargs["environment"] = {
            "POSTGRES_URL": "postgres://foreverbull:foreverbull@postgres:5432/foreverbull",
            "NATS_URL": "nats://nats:4222",
            "MINIO_URL": "minio:9000",
            "DOCKER_NETWORK": self.config.network_name,
            "BACKTEST_IMAGE": self.config.backtest_image,
            "LOG_LEVEL": "debug",
        }
        kwargs["healthcheck"] = {
            "test": ["CMD", "pg_isready", "-U", "foreverbull"],
            "interval": 10000000000,
            "timeout": 5000000000,
            "retries": 5,
        }
        kwargs["volumes"] = {
            "/var/run/docker.sock": {
                "bind": "/var/run/docker.sock",
                "mode": "rw",
            }
        }
        return super().create(*args, **kwargs)


class GrafanaContainer(BaseContainer):
    def __init__(self, config: Config, environment: Environment):
        super().__init__(config, environment)
        self.name = "foreverbull_grafana"
        self.hostname = "grafana"
        self.image = config.grafana_image

    def create(self, *args, **kwargs) -> ContainerStatus:
        kwargs["ports"] = {"3000/tcp": 3000}
        kwargs["environment"] = {"BROKER_URL": "foreverbull:50055"}
        return super().create(*args, **kwargs)


class ContainerManager:
    def __init__(self, environment: Environment, config: Config):
        self.config = config
        self.environment = environment
        self.postgres = PostgresContainer(config, environment)
        self.minio = MinioContainer(config, environment)
        self.nats = NatsContainer(config, environment)
        self.foreverbull = ForeverbullContainer(config, environment)
        self.grafana = GrafanaContainer(config, environment)
        self.containers = [self.postgres, self.minio, self.nats, self.foreverbull, self.grafana]

    def verify_images(self):
        for container in self.containers:
            container.get_or_download_image()

    def create(self):
        _docker.networks.create(self.config.network_name, driver="bridge")
        for container in self.containers:
            container.create()

    def start(self):
        for container in self.containers:
            container.start()

    def stop(self):
        for container in self.containers:
            container.stop()

    def remove(self):
        for container in self.containers:
            container.remove()
        _docker.networks.get(self.config.network_name).remove()
