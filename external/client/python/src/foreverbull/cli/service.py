import time

import typer
from rich.console import Console
from rich.progress import Progress
from rich.table import Table
from typing_extensions import Annotated

from foreverbull import broker, entity

image = Annotated[
    str,
    typer.Argument(help="service image"),
]

service = typer.Typer()

std = Console()
std_err = Console(stderr=True)


@service.command()
def list():
    services = broker.service.list()
    table = Table(title="Services")
    table.add_column("Image")
    table.add_column("Status")

    for service in services:
        table.add_row(
            service.image,
            (service.statuses[0].status.value if service.statuses else "Unknown"),
        )
    std.print(table)


@service.command()
def create(
    image: image,
):
    service: entity.service.Service | None = None
    with Progress() as progress:
        task = progress.add_task(
            "Starting service",
            total=2,
        )
        service = broker.service.create(image)
        previous_status = service.statuses[0]
        while not progress.finished:
            time.sleep(0.5)
            service = broker.service.get(image)
            status = service.statuses[0]
            if previous_status and previous_status.status != status.status:
                match status.status:
                    case entity.service.ServiceStatusType.INTERVIEW:
                        progress.advance(task)
                        progress.update(
                            task,
                            description="Interviewing service",
                        )
                    case entity.service.ServiceStatusType.READY:
                        progress.advance(task)
                        progress.update(
                            task,
                            description="Service ready",
                        )
                    case entity.service.ServiceStatusType.ERROR:
                        std_err.log(f"[red]Error while creating service: {status.error}")
                        exit(1)
                previous_status = status

    table = Table(title="Created Service")
    table.add_column("Image")
    table.add_column("Status")
    table.add_row(
        service.image,
        (service.statuses[0].status.value if service.statuses else "Unknown"),
    )
    std.print(table)


@service.command()
def get(
    image: image,
):
    service = broker.service.get(image)
    instances = broker.service.list_instances(image)

    table = Table(title="Service")
    table.add_column("Image")
    table.add_column("Status")
    table.add_row(
        service.image,
        (service.statuses[0].status.value if service.statuses else "Unknown"),
    )
    std.print(table)

    table = Table(title="Instances")
    table.add_column("ID")
    table.add_column("Hostname")
    table.add_column("Port")
    table.add_column("Status")
    table.add_column("Occurred At")
    for instance in instances:
        table.add_row(
            instance.id,
            instance.host,
            str(instance.port),
            (instance.statuses[0].status.value if instance.statuses else "Unknown"),
            (instance.statuses[0].occurred_at.isoformat() if instance.statuses else "Unknown"),
        )
    std.print(table)
