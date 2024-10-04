import json
import os
import tempfile
import time
from concurrent.futures import ThreadPoolExecutor, wait
from datetime import datetime

import docker.errors
import typer
from foreverbull import broker
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest import ingestion_pb2
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.table import Table
from typing_extensions import Annotated

import docker

version = "0.1.0"

env = typer.Typer()

std = Console()

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
    d = docker.from_env()

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
        foreverbull_image = d.images.get(BROKER_IMAGE)
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
    std.print(table)


ALPACA_KEY_OPT = Annotated[str, typer.Option(help="alpaca.markets api key")] | None
ALPACA_SECRET_OPT = Annotated[str, typer.Option(help="alpaca.markets api secret")] | None
BROKER_IMAGE_OPT = Annotated[str, typer.Option(help="Docker image name of broker")]
BACKTEST_IMAGE_OPT = Annotated[str, typer.Option(help="Docker image name of backtest service")]
INGESTION_CONFIG_OPT = Annotated[str, typer.Option(help="Path to ingestion config file")]


@env.command()
def start(
    alpaca_key: ALPACA_KEY_OPT = None,
    alpaca_secret: ALPACA_SECRET_OPT = None,
    broker_image: BROKER_IMAGE_OPT = BROKER_IMAGE,
    backtest_image: BACKTEST_IMAGE_OPT = BACKTEST_IMAGE,
    ingestion_config: INGESTION_CONFIG_OPT = "ingestion.json",
):
    d = docker.from_env()
    std.print("Starting environment")

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

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=False,
    ) as progress:
        download_images = progress.add_task("[yellow]Downloading images")
        net_task_id = progress.add_task("[yellow]Setting up network")
        postgres_task_id = progress.add_task("[yellow]Setting up postgres")
        nats_task_id = progress.add_task("[yellow]Setting up nats")
        minio_task_id = progress.add_task("[yellow]Setting up minio")
        health_task_id = progress.add_task("[yellow]Waiting for services to start")
        foreverbull_task_id = progress.add_task("[yellow]Setting up foreverbull")
        ingestion_task_id = progress.add_task("[yellow]Creating Ingestion")

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

        progress.update(download_images, description="[blue]Images downloaded", completed=True)

        try:
            d.networks.get(NETWORK_NAME)
        except docker.errors.NotFound:
            d.networks.create(NETWORK_NAME, driver="bridge")
        progress.update(net_task_id, description="[blue]Network created", completed=True)

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
                        }
                    },
                )
            except Exception as e:
                progress.update(
                    postgres_task_id,
                    description=f"[red]Failed to start postgres: {e}",
                    completed=True,
                )
                exit(1)
        progress.update(postgres_task_id, description="[blue]Postgres started", completed=True)

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
                    command="-js -sd /var/lib/nats/data",
                )
            except Exception as e:
                progress.update(
                    nats_task_id,
                    description=f"[red]Failed to start nats: {e}",
                    completed=True,
                )
                exit(1)
        progress.update(nats_task_id, description="[blue]NATS started", completed=True)

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
                    command='server --console-address ":9001" /data',
                )
            except Exception as e:
                progress.update(
                    minio_task_id,
                    description=f"[red]Failed to start minio: {e}",
                    completed=True,
                )
                exit(1)
        progress.update(minio_task_id, description="[blue]Minio started", completed=True)

        for _ in range(100):
            time.sleep(0.2)
            postgres_container = d.containers.get("foreverbull_postgres")
            if postgres_container.health != "healthy":
                continue
            nats_container = d.containers.get("foreverbull_nats")
            if nats_container.health != "healthy":
                continue
            progress.update(health_task_id, description="[blue]All services healthy", completed=True)
            break
        else:
            progress.update(
                health_task_id,
                description="[red]Failed to start services, timeout",
                completed=True,
            )
            exit(1)

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
                    },
                    environment={
                        "POSTGRES_URL": "postgres://foreverbull:foreverbull@postgres:5432/foreverbull",
                        "NATS_URL": "nats://nats:4222",
                        "MINIO_URL": "minio:9000",
                        "DOCKER_NETWORK": NETWORK_NAME,
                        "ALPACA_MARKETS_BASE_URL": "https://paper-api.alpaca.markets",
                        "ALPACA_MARKETS_API_KEY": alpaca_key,
                        "ALPACA_MARKETS_API_SECRET": alpaca_secret,
                        "BACKTEST_IMAGE": backtest_image,
                        "LOG_LEVEL": "info",
                    },
                    volumes={
                        "/var/run/docker.sock": {
                            "bind": "/var/run/docker.sock",
                            "mode": "rw",
                        }
                    },
                )
            except Exception as e:
                progress.update(
                    foreverbull_task_id,
                    description=f"[red]Failed to start foreverbull: {e}",
                    completed=True,
                )
                exit(1)
            time.sleep(2)
            try:
                with open(ingestion_config, "r") as f:
                    config = json.load(f)
                broker.backtest.ingest(
                    ingestion=ingestion_pb2.Ingestion(
                        start_date=pb_utils.to_proto_timestamp(datetime.strptime(config["start"], "%Y-%m-%d")),
                        end_date=pb_utils.to_proto_timestamp(datetime.strptime(config["end"], "%Y-%m-%d")),
                        symbols=config["symbols"],
                    )
                )
                for _ in range(60):
                    _, ingestion_status = broker.backtest.get_ingestion()
                    if ingestion_status == ingestion_pb2.IngestionStatus.READY:
                        break
                    time.sleep(1)
                else:
                    progress.update(
                        ingestion_task_id,
                        description="[red]Failed to ingest",
                        completed=True,
                    )
                    exit(1)
                progress.update(
                    ingestion_task_id,
                    description="[blue]Ingestion completed",
                    completed=True,
                )
            except Exception as e:
                progress.update(
                    ingestion_task_id,
                    description=f"[red]Failed to ingest: {e}",
                    completed=True,
                )
                exit(1)
        progress.update(foreverbull_task_id, description="[blue]Foreverbull started", completed=True)
    std.print("Environment started")


@env.command()
def stop():
    d = docker.from_env()
    std.print("Stopping environment")
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
    ) as progress:
        net_task_id = progress.add_task("[yellow]Tearing down network")
        postgres_task_id = progress.add_task("[yellow]Tearing down postgres")
        nats_task_id = progress.add_task("[yellow]Tearing down nats")
        minio_task_id = progress.add_task("[yellow]Tearing down minio")
        foreverbull_task_id = progress.add_task("[yellow]Tearing down foreverbull")

        try:
            d.containers.get("foreverbull_foreverbull").stop()
            d.containers.get("foreverbull_foreverbull").remove()
        except docker.errors.NotFound:
            pass
        progress.update(foreverbull_task_id, description="[blue]Foreverbull removed", completed=True)

        try:
            d.containers.get("foreverbull_minio").stop()
            d.containers.get("foreverbull_minio").remove()
        except docker.errors.NotFound:
            pass
        progress.update(minio_task_id, description="[blue]Minio removed", completed=True)

        try:
            d.containers.get("foreverbull_nats").stop()
            d.containers.get("foreverbull_nats").remove()
        except docker.errors.NotFound:
            pass
        progress.update(nats_task_id, description="[blue]NATS removed", completed=True)

        try:
            d.containers.get("foreverbull_postgres").stop()
            d.containers.get("foreverbull_postgres").remove()
        except docker.errors.NotFound:
            pass
        progress.update(postgres_task_id, description="[blue]Postgres removed", completed=True)

        try:
            d.networks.get(NETWORK_NAME).remove()
        except docker.errors.NotFound:
            pass
        progress.update(net_task_id, description="[blue]Network removed", completed=True)
    std.print("Environment stopped")
