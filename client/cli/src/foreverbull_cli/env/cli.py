import typer

from grpc._channel import _InactiveRpcError
from rich.progress import Progress
from rich.progress import TextColumn
from rich.table import Table

from foreverbull_cli.env.environment import Config
from foreverbull_cli.env.environment import ContainerManager
from foreverbull_cli.env.environment import Environment
from foreverbull_cli.output import FBSpinnerColumn
from foreverbull_cli.output import console


class FBProgress(Progress):
    def __init__(self):
        spinner = FBSpinnerColumn()
        text = TextColumn("[progress.description]{task.description}")
        super().__init__(spinner, text)

    def __enter__(self) -> "FBProgress":
        return super().__enter__()

    def __exit__(self, exc_type, exc_val, exc_tb):
        if isinstance(exc_val, _InactiveRpcError):
            details = exc_val.details()
        else:
            details = exc_val

        for task in self.tasks:
            if task.total == 2 and task.completed == 1:
                self.update(task.id, description=f"{task.description} FAILED: {details}")
                self.update(task.id, completed=-1)

        super().__exit__(exc_type, exc_val, exc_tb)


cli = typer.Typer()


def print_status(cm: ContainerManager):
    table = Table(title="Environment Status")
    table.add_column("Service")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Version")
    table.add_column("Container ID")
    for row in cm.status_rows():
        table.add_row(row.service, row.name, row.status, row.image, row.container_id)
    console.print(table)


@cli.command(help="Status of the environment")
def status(ctx: typer.Context):
    cm: ContainerManager = ctx.obj
    print_status(cm)


@cli.command(help="create new environment")
def create(ctx: typer.Context, start: bool = True):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        verify_images = progress.add_task("Verifying images", total=2)
        create_task = progress.add_task("Creating environment", total=2)
        if start:
            start_task = progress.add_task("Starting environment", total=2)
        else:
            start_task = None

        progress.update(verify_images, completed=1)
        cm.verify_images()
        progress.update(verify_images, completed=2)

        progress.update(create_task, completed=1)
        cm.create(start=start)

        progress.update(create_task, completed=2)
        if start_task:
            progress.update(start_task, completed=1)
            cm.start()
            progress.update(start_task, completed=2)

    print_status(cm)


@cli.command(help="start environment")
def start(ctx: typer.Context):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        start_task = progress.add_task("Starting environment", total=2)
        progress.update(start_task, completed=1)
        cm.start()
        progress.update(start_task, completed=2)

    print_status(cm)


@cli.command(help="stop environment")
def stop(
    ctx: typer.Context,
    remove: bool = typer.Option(False, "--remove", help="Uninstall the Helm release"),
    purge: bool = typer.Option(
        False,
        "--purge",
        help="With --remove, delete the Kind cluster (local) and .foreverbull data",
    ),
):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        stop_task = progress.add_task("Stopping environment", total=2)
        if remove:
            remove_task = progress.add_task("Removing environment", total=2)
        else:
            remove_task = None

        progress.update(stop_task, completed=1)
        cm.stop()
        progress.update(stop_task, completed=2)

        if remove_task:
            progress.update(remove_task, completed=1)
            cm.remove(purge)
            progress.update(remove_task, completed=2)
    print_status(cm)


@cli.command(help="update environment images")
def update(ctx: typer.Context):
    cm: ContainerManager = ctx.obj

    with FBProgress() as progress:
        update_task = progress.add_task("Updating environment", total=2)
        progress.update(update_task, completed=1)
        cm.update()
        progress.update(update_task, completed=2)

    print_status(cm)


@cli.callback()
def initialize(ctx: typer.Context):
    config = Config()
    environment = Environment()
    ctx.obj = ContainerManager(environment, config)
