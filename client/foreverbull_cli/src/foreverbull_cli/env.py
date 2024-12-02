import os
import tempfile
import time

from concurrent.futures import ThreadPoolExecutor
from concurrent.futures import wait
from pathlib import Path

import docker
import docker.errors
import typer

from rich.table import Table
from typing_extensions import Annotated

from foreverbull_cli.output import FBProgress
from foreverbull_cli.output import console


version = "0.1.0"

env = typer.Typer()


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


_environment: Environment


@env.callback()
def setup_path(
    ctx: typer.Context,
    path: str = typer.Option(None, "-p", help="Path to foreverbull configuration"),
):
    global _environment
    _environment = Environment(path)


INIT_DB_SCIPT = """
#!/bin/bash

set -e
set -u

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
	CREATE USER foreverbull WITH PASSWORD 'foreverbull';
	ALTER ROLE foreverbull Superuser;

	CREATE DATABASE foreverbull;
	GRANT ALL PRIVILEGES ON DATABASE foreverbull TO foreverbull;
	ALTER DATABASE foreverbull OWNER TO foreverbull;

	CREATE DATABASE foreverbull_testing;
	    GRANT ALL PRIVILEGES ON DATABASE foreverbull_testing TO foreverbull;
	    ALTER DATABASE foreverbull_testing OWNER TO foreverbull;
EOSQL
"""

NETWORK_NAME = "foreverbull"

POSTGRES_IMAGE = "postgres:13.3-alpine"
NATS_IMAGE = "nats:2.10-alpine"
MINIO_IMAGE = "minio/minio:latest"
BROKER_IMAGE = f"lhjnilsson/foreverbull:{version}"
BACKTEST_IMAGE = f"lhjnilsson/zipline:{version}"


@env.command()
def status():
    try:
        d = docker.from_env()
    except docker.errors.DockerException as e:
        console.print(f"[red]Failed to connect to Docker: [yellow]{e}")
        return

    try:
        postgres_container = d.containers.get("foreverbull_postgres")
    except docker.errors.NotFound:
        postgres_container = None
    try:
        nats_container = d.containers.get("foreverbull_nats")
    except docker.errors.NotFound:
        nats_container = None
    try:
        minio_container = d.containers.get("foreverbull_minio")
    except docker.errors.NotFound:
        minio_container = None
    try:
        foreverbull_container = d.containers.get("foreverbull_foreverbull")
    except docker.errors.NotFound:
        foreverbull_container = None

    try:
        postgres_image = d.images.get(POSTGRES_IMAGE)
    except docker.errors.ImageNotFound:
        postgres_image = None
    try:
        nats_image = d.images.get(NATS_IMAGE)
    except docker.errors.ImageNotFound:
        nats_image = None
    try:
        minio_image = d.images.get(MINIO_IMAGE)
    except docker.errors.ImageNotFound:
        minio_image = None
    try:
        foreverbull_image = d.images.get(BROKER_IMAGE)  # type: ignore
    except docker.errors.ImageNotFound:
        foreverbull_image = None

    table = Table(title="Environment Status")
    table.add_column("Status")
    table.add_column("Service")
    table.add_column("Local image ID")

    table.add_row(
        postgres_container.status if postgres_container else "Not Found",
        "Postgres",
        postgres_image.short_id if postgres_image else "Not found",
    )
    table.add_row(
        nats_container.status if nats_container else "Not Found",
        "NATS",
        nats_image.short_id if nats_image else "Not found",
    )
    table.add_row(
        minio_container.status if minio_container else "Not Found",
        "Minio",
        minio_image.short_id if minio_image else "Not found",
    )
    table.add_row(
        foreverbull_container.status if foreverbull_container else "Not Found",
        "Foreverbull",
        foreverbull_image.short_id if foreverbull_image else "Not found",
    )
    console.print(table)


@env.command()
def start(
    broker_image: Annotated[str, typer.Option(help="Docker image name of broker")] = BROKER_IMAGE,
    backtest_image: Annotated[str, typer.Option(help="Docker image name of backtest service")] = BACKTEST_IMAGE,
    log_level: Annotated[str, typer.Option(help="Log level")] = "INFO",
):
    try:
        d = docker.from_env()
    except docker.errors.DockerException as e:
        console.print(f"[red]Failed to connect to Docker: [yellow]{e}")
        return

    def get_or_pull_image(image_name):
        try:
            d.images.get(image_name)
        except docker.errors.ImageNotFound:
            try:
                d.images.pull(image_name)
            except Exception as e:
                return e
        except Exception as e:
            return e
        return None

    with FBProgress() as progress:
        download_images = progress.add_task("Download Images", total=2)
        net_task_id = progress.add_task("Creating Environment", total=2)
        postgres_task_id = progress.add_task("Postgres", total=2)
        nats_task_id = progress.add_task("NATS", total=2)
        minio_task_id = progress.add_task("Minio", total=2)
        health_task_id = progress.add_task("Waiting for services to start", total=2)
        foreverbull_task_id = progress.add_task("Foreverbull", total=2)

        progress.update(download_images, completed=1)
        with ThreadPoolExecutor() as executor:
            futures = []
            for image in [
                POSTGRES_IMAGE,
                NATS_IMAGE,
                MINIO_IMAGE,
                broker_image,
                backtest_image,
            ]:
                futures.append(executor.submit(get_or_pull_image, image))
                wait(futures)
            for future in futures:
                if future.result():
                    progress.update(
                        download_images,
                        description=f"[red]Failed to download images: {future.result()}",
                    )
                    exit(1)
        progress.update(download_images, completed=2)

        progress.update(net_task_id, completed=1)
        try:
            d.networks.get(NETWORK_NAME)
        except docker.errors.NotFound:
            d.networks.create(NETWORK_NAME, driver="bridge")
        progress.update(net_task_id, completed=2)

        progress.update(postgres_task_id, completed=1)
        try:
            postgres_container = d.containers.get("foreverbull_postgres")
            if postgres_container.status != "running":
                postgres_container.start()
            if postgres_container.health != "healthy":
                postgres_container.restart()
        except docker.errors.NotFound:
            try:
                init_db_file = tempfile.NamedTemporaryFile(delete=False)
                init_db_file.write(INIT_DB_SCIPT.encode())
                init_db_file.close()
                os.chmod(init_db_file.name, 0o777)

                postgres_container = d.containers.run(
                    POSTGRES_IMAGE,
                    name="foreverbull_postgres",
                    detach=True,
                    network=NETWORK_NAME,
                    hostname="postgres",
                    ports={"5432/tcp": 5432},
                    environment={
                        "POSTGRES_PASSWORD": "foreverbull",
                        "PGDATA": "/pgdata",
                    },
                    healthcheck={
                        "test": ["CMD", "pg_isready", "-U", "foreverbull"],
                        "interval": 10000000000,
                        "timeout": 5000000000,
                        "retries": 5,
                    },
                    volumes={
                        init_db_file.name: {
                            "bind": "/docker-entrypoint-initdb.d/init.sh",
                            "mode": "ro",
                        },
                        str((_environment.postgres_location / "data").absolute()): {
                            "bind": "/pgdata",
                            "mode": "rw",
                        },
                    },
                )
            except Exception as e:
                progress.update(
                    postgres_task_id,
                    description=f"[red]Failed to start postgres: {e}",
                    completed=100,
                )
                exit(1)
        progress.update(postgres_task_id, completed=2)

        progress.update(nats_task_id, completed=1)
        try:
            nats_container = d.containers.get("foreverbull_nats")
            if nats_container.status != "running":
                nats_container.start()
            if nats_container.health != "healthy":
                nats_container.restart()
        except docker.errors.NotFound:
            try:
                nats_container = d.containers.run(
                    NATS_IMAGE,
                    name="foreverbull_nats",
                    detach=True,
                    network=NETWORK_NAME,
                    hostname="nats",
                    ports={"4222/tcp": 4222},
                    healthcheck={
                        "test": ["CMD", "nats-server", "-sl"],
                        "interval": 10000000000,
                        "timeout": 5000000000,
                        "retries": 5,
                    },
                    volumes={
                        str((_environment.nats_location / "data").absolute()): {
                            "bind": "/var/lib/nats/data",
                            "mode": "rw",
                        },
                    },
                    command="-js -sd /var/lib/nats/data",
                )
            except Exception as e:
                progress.update(
                    nats_task_id,
                    description=f"[red]Failed to start nats: {e}",
                    completed=2,
                )
                exit(1)
        progress.update(nats_task_id, completed=2)

        progress.update(minio_task_id, completed=1)
        try:
            d.containers.get("foreverbull_minio")
        except docker.errors.NotFound:
            try:
                d.containers.run(
                    MINIO_IMAGE,
                    name="foreverbull_minio",
                    detach=True,
                    network=NETWORK_NAME,
                    hostname="minio",
                    ports={"9000/tcp": 9000},
                    volumes={
                        str((_environment.minio_location / "data").absolute()): {
                            "bind": "/data",
                            "mode": "rw",
                        },
                    },
                    command='server --console-address ":9001" /data',
                )
            except Exception as e:
                progress.update(
                    minio_task_id,
                    description=f"[red]Failed to start minio: {e}",
                    completed=2,
                )
                exit(1)
        progress.update(minio_task_id, completed=2)

        progress.update(health_task_id, completed=1)
        for _ in range(100):
            time.sleep(0.2)
            postgres_container = d.containers.get("foreverbull_postgres")
            if postgres_container.health != "healthy":
                continue
            nats_container = d.containers.get("foreverbull_nats")
            if nats_container.health != "healthy":
                continue
            progress.update(health_task_id, completed=2)
            break
        else:
            progress.update(
                health_task_id,
                description="[red]Failed to start services, timeout",
                completed=2,
            )
            exit(1)

        progress.update(foreverbull_task_id, completed=1)
        try:
            foreverbull_container = d.containers.get("foreverbull_foreverbull")
            if foreverbull_container.status != "running":
                foreverbull_container.start()
        except docker.errors.NotFound:
            try:
                d.containers.run(
                    broker_image,
                    name="foreverbull_foreverbull",
                    detach=True,
                    network=NETWORK_NAME,
                    hostname="foreverbull",
                    ports={
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
                    },
                    environment={
                        "POSTGRES_URL": "postgres://foreverbull:foreverbull@postgres:5432/foreverbull",
                        "NATS_URL": "nats://nats:4222",
                        "MINIO_URL": "minio:9000",
                        "DOCKER_NETWORK": NETWORK_NAME,
                        "BACKTEST_IMAGE": backtest_image,
                        "LOG_LEVEL": log_level,
                    },  # type: ignore
                    volumes={
                        "/var/run/docker.sock": {
                            "bind": "/var/run/docker.sock",
                            "mode": "rw",
                        }
                    },
                )  # type: ignore
            except Exception as e:
                progress.update(
                    foreverbull_task_id,
                    description=f"[red]Failed to start foreverbull: {e}",
                    completed=100,
                )
                exit(1)
            time.sleep(2)
        progress.update(foreverbull_task_id, completed=2)


@env.command()
def stop():
    try:
        d = docker.from_env()
    except docker.errors.DockerException as e:
        console.print(f"[red]Failed to connect to Docker: [yellow]{e}")
        return
    with FBProgress() as progress:
        foreverbull_task_id = progress.add_task("Foreverbull", total=2)
        minio_task_id = progress.add_task("Minio", total=2)
        nats_task_id = progress.add_task("NATS", total=2)
        postgres_task_id = progress.add_task("Postgres", total=2)
        net_task_id = progress.add_task("Removing Environment", total=2)

        progress.update(foreverbull_task_id, completed=1)
        try:
            d.containers.get("foreverbull_foreverbull").stop()
            d.containers.get("foreverbull_foreverbull").remove()
        except docker.errors.NotFound:
            pass
        progress.update(foreverbull_task_id, completed=2)

        progress.update(minio_task_id, completed=1)
        try:
            d.containers.get("foreverbull_minio").stop()
            d.containers.get("foreverbull_minio").remove()
        except docker.errors.NotFound:
            pass
        progress.update(minio_task_id, completed=2)

        progress.update(minio_task_id, completed=1)
        try:
            d.containers.get("foreverbull_nats").stop()
            d.containers.get("foreverbull_nats").remove()
        except docker.errors.NotFound:
            pass
        progress.update(nats_task_id, completed=2)

        progress.update(postgres_task_id, completed=1)
        try:
            d.containers.get("foreverbull_postgres").stop()
            d.containers.get("foreverbull_postgres").remove()
        except docker.errors.NotFound:
            pass
        progress.update(postgres_task_id, completed=2)

        progress.update(net_task_id, completed=1)
        try:
            d.networks.get(NETWORK_NAME).remove()
        except docker.errors.NotFound:
            pass
        progress.update(net_task_id, completed=2)
