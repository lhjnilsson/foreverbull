from datetime import datetime, date

import typer
from foreverbull import broker
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.pb_utils import from_proto_date_to_pydate, from_pydate_to_proto_date
from rich.console import Console
from rich.table import Table
from typing_extensions import Annotated
import json
from pathlib import Path

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
        name = config_file.name
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
    pass

"""
@backtest.command()
def run(
    file_path: Annotated[str, typer.Argument(help="name of the file to use")],
    backtest_name: Annotated[str, typer.Option(help="name of the backtest")],
):
    def show_progress(session: entity.backtest.Session):
        with Progress() as progress:
            task = progress.add_task("Starting", total=2)
            previous_status = session.statuses[0].status
            while not progress.finished:
                time.sleep(0.5)
                session = broker.backtest.get_session(session.id)
                status = session.statuses[0].status
                if previous_status and previous_status != status:
                    match status:
                        case entity.backtest.SessionStatusType.RUNNING:
                            progress.advance(task)
                            progress.update(task, description="Running")
                        case entity.backtest.SessionStatusType.COMPLETED:
                            progress.advance(task)
                            progress.update(task, description="Completed")
                        case entity.backtest.SessionStatusType.FAILED:
                            std_err.log(f"[red]Error while running session: {session.statuses[0].error}")
                            exit(1)
                    previous_status = status

        table = Table(title="Session")
        table.add_column("Id")
        table.add_column("Status")
        table.add_column("Date")
        table.add_column("Executions")
        table.add_row(
            session.id,
            session.statuses[0].status.value if session.statuses else "Unknown",
            session.statuses[0].occurred_at.isoformat() if session.statuses else "Unknown",
            str(session.executions),
        )
        std.print(table)

    algorithm = Algorithm.from_file_path(file_path)
    with algorithm.backtest_session(backtest_name) as session:
        default = session.get_default()
        session.run_execution(
            start=default.start,
            end=default.end,
            symbols=default.symbols,
            benchmark=default.benchmark,
        )
"""
