import json
import logging
import time

from datetime import date
from pathlib import Path

import typer

from rich.live import Live
from rich.progress import BarColumn
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import TextColumn
from rich.table import Table
from typing_extensions import Annotated

from foreverbull import Algorithm
from foreverbull import broker
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.backtest import ingestion_pb2
from foreverbull.pb.pb_utils import from_proto_date_to_pydate
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.output import console


backtest = typer.Typer()
log = logging.getLogger().getChild(__name__)


@backtest.command()
def list():
    table = Table(title="Backtests")
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")
    for backtest in broker.backtest.list_backtests():
        table.add_row(
            backtest.name,
            (from_proto_date_to_pydate(backtest.start_date).isoformat()),
            (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.HasField("end_date") else None),
            ",".join(backtest.symbols),
            backtest.benchmark,
        )
    console.print(table)


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
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")

    table.add_row(
        backtest.name,
        (from_proto_date_to_pydate(backtest.start_date).isoformat() if backtest.start_date else ""),
        (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.end_date else ""),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    console.print(table)


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
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")
    table.add_row(
        backtest.name,
        (backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status) if backtest.statuses else "Unknown"),
        from_proto_date_to_pydate(backtest.start_date).isoformat(),
        (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.HasField("end_date") else None),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    console.print(table)


@backtest.command()
def ingest():
    broker.backtest.ingest()
    for _ in range(90):
        _, ingestion_status = broker.backtest.get_ingestion()
        if ingestion_status == ingestion_pb2.IngestionStatus.READY:
            console.print("Ingestion completed")
            break
        time.sleep(1)
    else:
        log.error("[red]Ingestion failed")
        exit(1)


@backtest.command()
def run(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
    file_path: Annotated[str, typer.Argument(help="name of the backtest")],
    start: Annotated[str, typer.Option(help="start date of the backtest")] | None = None,
    end: Annotated[str, typer.Option(help="end date of the backtest")] | None = None,
):
    progress = Progress(
        SpinnerColumn(),
        "[progress.description]{task.description}",
        BarColumn(),
        "[progress.percentage]{task.percentage:>3.0f}%",
        TextColumn("[progress.completed]"),
    )
    live = Live(progress, console=console, refresh_per_second=120)

    with Algorithm.from_file_path(file_path).backtest_session(name) as session, live:
        backtest = session.get_default()

        start_date = backtest.start_date
        if start:
            try:
                start_date = from_pydate_to_proto_date(date.fromisoformat(start))
            except Exception:
                console.print(f"[red]Invalid start date: [yellow]{start}")
                return

        end_date = backtest.end_date
        if end:
            try:
                end_date = from_pydate_to_proto_date(date.fromisoformat(end))
            except Exception:
                console.print(f"[red]Invalid end date: [yellow]{end}")
                return

        total_months = (end_date.year - start_date.year) * 12 + end_date.month - start_date.month
        log.info(f"Running backtest {backtest.name} from. Total Months: {total_months}")

        task = progress.add_task(f"{backtest.name}", total=total_months)
        current_month = start_date.month
        for period in session.run_execution(
            start_date,
            end_date,
            [s for s in backtest.symbols],
        ):
            if period.timestamp.ToDatetime().month != current_month:
                progress.update(task, advance=1)
                current_month = period.timestamp.ToDatetime().month
        log.info(f"Execution completed for {backtest.name}")


@backtest.command()
def executions(
    backtest: Annotated[str, typer.Option(help="name of the backtest")] | None = None,
    session: Annotated[str, typer.Option(help="id of the session")] | None = None,
):
    executions = broker.backtest.list_executions(backtest, session)

    table = Table()
    table.add_column("Date")
    table.add_column("Status")
    table.add_column("Backtest")
    table.add_column("ID")

    for execution in executions:
        table.add_row(
            execution.statuses[0].occurred_at.ToDatetime().strftime("%Y-%m-%d %H:%M:%S"),
            execution_pb2.Execution.Status.Status.Name(execution.statuses[0].status),
            execution.backtest,
            execution.id,
        )

    console.print(table)


@backtest.command()
def execution(
    id: Annotated[str, typer.Argument(help="id of the execution")],
):
    _, periods = broker.backtest.get_execution(id)

    table = Table()
    table.add_column("Date")
    table.add_column("Portfolio Value")
    table.add_column("Benchmark Period Return")
    table.add_column("Benchmark Volatility")
    table.add_column("Alpha")
    table.add_column("Beta")
    table.add_column("Sharpe")
    table.add_column("Sortino")

    for period in periods:
        table.add_row(
            from_proto_date_to_pydate(period.date).strftime("%Y-%m-%d"),
            "{:.5f}".format(period.portfolio_value),
            "{:.5f}".format(period.benchmark_period_return),
            "{:.5f}".format(period.benchmark_volatility),
            "{:.5f}".format(period.alpha),
            "{:.5f}".format(period.beta),
            "{:.5f}".format(period.sharpe),
            "{:.5f}".format(period.sortino),
        )
    console.print(table)
