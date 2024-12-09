import json
import logging

from datetime import date
from pathlib import Path

import typer

from rich.live import Live
from rich.progress import BarColumn
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import TextColumn
from typing_extensions import Annotated

from foreverbull import Algorithm
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.output import console


strategy = typer.Typer()
log = logging.getLogger().getChild(__name__)


@strategy.command()
def run(
    file_path: Annotated[str, typer.Argument(help="name of the strategy")],
    config: Annotated[str, typer.Argument(help="path to the config file")],
):
    config_file = Path(config)
    with open(config_file, "r") as f:
        cfg = json.load(f)

    from_date = from_pydate_to_proto_date(date.fromisoformat(cfg["start_date"]))

    progress = Progress(
        SpinnerColumn(),
        "[progress.description]{task.description}",
        BarColumn(),
        "[progress.percentage]{task.percentage:>3.0f}%",
        TextColumn("[progress.completed]"),
    )
    live = Live(progress, console=console, refresh_per_second=120)
    algorithm = Algorithm.from_file_path(file_path)

    with live:
        algorithm.run_strategy(from_date, cfg["symbols"])
