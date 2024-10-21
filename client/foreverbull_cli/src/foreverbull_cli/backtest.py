from datetime import date
import time
import typer
from foreverbull import broker
from foreverbull.pb.foreverbull.backtest import backtest_pb2, ingestion_pb2
from foreverbull.pb.pb_utils import from_proto_date_to_pydate, from_pydate_to_proto_date
from rich.console import Console
from rich.table import Table
from typing_extensions import Annotated
import json
from pathlib import Path
from foreverbull import Algorithm

backtest = typer.Typer()

std = Console()
std_err = Console(stderr=True)


@backtest.command()
def list():
    table = Table(title="Backtests")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")
    for backtest in broker.backtest.list():
        table.add_row(
            backtest.name,
            (backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status) if backtest.statuses else "Unknown"),
            (from_proto_date_to_pydate(backtest.start_date).isoformat()),
            (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.HasField("end_date") else None),
            ",".join(backtest.symbols),
            backtest.benchmark,
        )
    std.print(table)


@backtest.command()
def create(
    config: Annotated[str, typer.Argument(help="path to the config file")],
    name: Annotated[str, typer.Option(help="name of the backtest, filename if None")] | None = None,
):
    config_file = Path(config)
    with open(config_file, "r") as f:
        cfg = json.load(f)

    assert "start_date" in cfg, "start_date is required in config"
    assert "symbols" in cfg, "symbols is required in config"
    if name is None:
        name = config_file.stem
    start = date.fromisoformat(cfg["start_date"])
    end = date.fromisoformat(cfg.get("end_date")) if "end_date" in cfg else None

    backtest = backtest_pb2.Backtest(
        name=name,
        start_date=from_pydate_to_proto_date(start),
        end_date=from_pydate_to_proto_date(end) if end else None,
        symbols=cfg["symbols"],
        benchmark=cfg.get("benchmark"),
    )
    backtest = broker.backtest.create(backtest)
    table = Table(title="Created Backtest")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")

    table.add_row(
        backtest.name,
        (backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status) if backtest.statuses else "Unknown"),
        (from_proto_date_to_pydate(backtest.start_date).isoformat() if backtest.start_date else ""),
        (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.end_date else ""),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    std.print(table)


@backtest.command()
def get(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
):
    backtest = broker.backtest.get(name)
    table = Table(title="Backtest")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")
    table.add_row(
        backtest.name,
        (backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status) if backtest.statuses else "Unknown"),
        from_proto_date_to_pydate(backtest.start_date).isoformat(),
        (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.HasField("end_date") else None),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    std.print(table)


@backtest.command()
def ingest():
    broker.backtest.ingest()
    for _ in range(60):
        _, ingestion_status = broker.backtest.get_ingestion()
        if ingestion_status == ingestion_pb2.IngestionStatus.READY:
            std.print("Ingestion completed")
            break
        time.sleep(1)
    else:
        std_err.log("[red]Ingestion failed")
        exit(1)

import time
import logging
from rich.progress import Progress, SpinnerColumn, BarColumn, TextColumn
from rich.logging import RichHandler
from rich.live import Live
from rich.console import Console
from rich.panel import Panel
from rich.layout import Layout
from rich.text import Text

class LoggingHandler(logging.Handler):
    def __init__(self, layout, name):
        super().__init__()
        self.layout = layout
        self.name = name
        self.console = Console(record=True)
        self.log_buffer = []

    def emit(self, record):
        log_entry = self.format(record)
        self.log_buffer.append(log_entry)
        self.console.clear()
        for entry in self.log_buffer:
            self.console.print(entry)
        self.layout[self.name].update(Panel(self.console.export_text()))


@backtest.command()
def run(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
    file_path: Annotated[str, typer.Argument(help="name of the backtest")],
):
    algo = Algorithm.from_file_path(file_path)
    std.print(f"Running backtest {name} with algorithm {file_path}")

    console = Console()
    progress = Progress(
        SpinnerColumn(),
        "[progress.description]{task.description}",
        BarColumn(),
        "[progress.percentage]{task.percentage:>3.0f}%",
        TextColumn("[progress.completed]{task.completed}/{task.total}"),
    )

    layout = Layout()
    layout.split(
        Layout(name="logs", ratio=3),
        Layout(name="progress", ratio=1)
    )
    layout_handler = LoggingHandler(layout, "logs")
    layout_handler.setLevel(logging.INFO)
    layout_handler.setFormatter(logging.Formatter("%(message)s"))


    logger = logging.getLogger("rich")
    logger.setLevel(logging.DEBUG)
    logger.addHandler(layout_handler)

    with algo.backtest_session(name) as session, Live(layout, refresh_per_second=4):
        task = progress.add_task("Processing...", total=100)
        layout["progress"].update(Panel(progress, width=console.width))

        backtest = session.get_default()
        std.print(f"Running Execution for {backtest.name}")
        for period in session.run_execution(
            backtest.start_date,
            backtest.end_date,
            [s for s in backtest.symbols],
        ):
            pass
        logger.info(f"Execution completed for {backtest.name}")
