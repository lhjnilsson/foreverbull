import os

import typer

from rich.table import Table

from foreverbull_cli.env.environment import Config
from foreverbull_cli.env.environment import ContainerManager
from foreverbull_cli.env.environment import Environment
from foreverbull_cli.output import FBProgress
from foreverbull_cli.output import console


cli = typer.Typer()


def print_status(cm: ContainerManager):
    table = Table(title="Environment Status")
    table.add_column("Service")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Version")
    table.add_column("Container ID")
    table.add_row(
        "Postgres", cm.postgres.name, cm.postgres.status(), cm.postgres.image_version(), cm.postgres.container_id()
    )
    table.add_row("NATS", cm.nats.name, cm.nats.status(), cm.nats.image_version(), cm.nats.container_id())
    table.add_row("Minio", cm.minio.name, cm.minio.status(), cm.minio.image_version(), cm.minio.container_id())
    table.add_row(
        "Foreverbull",
        cm.foreverbull.name,
        cm.foreverbull.status(),
        cm.foreverbull.image_version(),
        cm.foreverbull.container_id(),
    )
    table.add_row(
        "Grafana", cm.grafana.name, cm.grafana.status(), cm.grafana.image_version(), cm.grafana.container_id()
    )
    console.print(table)


@cli.command(help="Status of the environment")
def status(ctx: typer.Context):
    cm: ContainerManager = ctx.obj
    print_status(cm)


@cli.command(help="create new environment")
def create(ctx: typer.Context, start: bool = True, version: str = "latest"):
    cm: ContainerManager = ctx.obj

    os.environ["version"] = version
    with FBProgress() as progress:
        verify_images = progress.add_task("Verifying images", total=2)
        create = progress.add_task("Creating environment", total=2)
        if start:
            start_task = progress.add_task("Starting environment", total=2)
        else:
            start_task = None

        progress.update(verify_images, completed=1)
        cm.verify_images()
        progress.update(verify_images, completed=2)

        progress.update(create, completed=1)
        cm.create()

        progress.update(create, completed=2)
        if start_task:
            progress.update(start_task, completed=1)
            cm.start()
            progress.update(start_task, completed=2)

    print_status(cm)


@cli.command(help="start environment")
def start(ctx: typer.Context):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        start = progress.add_task("Starting environment", total=2)
        progress.update(start, completed=1)
        cm.start()
        progress.update(start, completed=2)

    print_status(cm)


@cli.command(help="stop environment")
def stop(ctx: typer.Context, remove: bool = False):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        stop = progress.add_task("Stopping environment", total=2)
        if remove:
            remove_task = progress.add_task("Removing environment", total=2)
        else:
            remove_task = None
        progress.update(stop, completed=1)
        cm.stop()
        progress.update(stop, completed=2)

        if remove_task:
            progress.update(remove_task, completed=1)
            cm.remove()
            progress.update(remove_task, completed=2)

    print_status(cm)


@cli.command(help="update environment images")
def update(ctx: typer.Context):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        update = progress.add_task("Updating environment", total=2)
        progress.update(update, completed=1)
        import time

        time.sleep(1)
        progress.update(update, completed=2)

    print_status(cm)


@cli.callback()
def initialize(ctx: typer.Context):
    config = Config()
    environment = Environment()
    ctx.obj = ContainerManager(environment, config)


if __name__ == "__main__":
    cli()
