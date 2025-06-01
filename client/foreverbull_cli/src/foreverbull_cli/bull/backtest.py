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
from rich.table import Table
from typing_extensions import Annotated

from foreverbull import broker
from foreverbull import models
from foreverbull.foreverbull import Foreverbull
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.pb_utils import from_proto_date_to_pydate
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.output import FBProgress
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
    name: Annotated[str, typer.Option(help="name of the backtest, filename if None")] = "",
):
    config_file = Path(config)
    with open(config_file, "r") as f:
        cfg = json.load(f)

    assert "start_date" in cfg, "start_date is required in config"
    assert "symbols" in cfg, "symbols is required in config"
    if not name:
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
    backtest_rsp = backtest_service_pb2.CreateBacktestResponse()
    with FBProgress() as progress:
        create_task = progress.add_task("Creating Backtest", total=2)

        progress.update(create_task, completed=1)
        rsp = broker.backtest.create(backtest)
        for backtest_rsp in rsp:
            status = backtest_pb2.BacktestStatus.Name(backtest_rsp.status.status)
            state = common_pb2.State.Name(backtest_rsp.status.state)
            progress.update(
                create_task,
                description=f"Creating Backtest: {status} - {state}",
            )
        progress.update(create_task, completed=2)

    table = Table(title="Created Backtest", expand=True)
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")
    table.add_row(
        backtest_rsp.backtest.name,
        (
            from_proto_date_to_pydate(backtest_rsp.backtest.start_date).isoformat()
            if backtest_rsp.backtest.start_date
            else ""
        ),
        (
            from_proto_date_to_pydate(backtest_rsp.backtest.end_date).isoformat()
            if backtest_rsp.backtest.end_date
            else ""
        ),
        ",".join(backtest_rsp.backtest.symbols),
        backtest_rsp.backtest.benchmark,
    )
    console.print(table)


@backtest.command()
def get(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
):
    backtest = broker.backtest.get(name)
    backtest_table = Table(title="Backtest", expand=True)
    backtest_table.add_column("Name")
    backtest_table.add_column("Start")
    backtest_table.add_column("End")
    backtest_table.add_column("Symbols", overflow="fold")
    backtest_table.add_column("Benchmark")
    backtest_table.add_row(
        backtest.name,
        from_proto_date_to_pydate(backtest.start_date).isoformat(),
        (from_proto_date_to_pydate(backtest.end_date).isoformat() if backtest.HasField("end_date") else None),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    executions = broker.backtest.list_executions(name, None)
    executions_table = Table(title="executions", expand=True)
    executions_table.add_column("Start")
    executions_table.add_column("End")
    executions_table.add_column("Link")
    for execution in executions:
        executions_table.add_row(
            execution.backtest,
            execution.id,
        )

    console.print(backtest_table)
    console.print(executions_table)


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
    with Foreverbull(models.Algorithm.from_file_path(file_path)).backtest_session(name) as session, live:
        start_date = session.backtest.start_date
        if start:
            try:
                start_date = from_pydate_to_proto_date(date.fromisoformat(start))
            except Exception:
                console.print(f"[red]Invalid start date: [yellow]{start}")
                return

        end_date = session.backtest.end_date
        if end:
            try:
                end_date = from_pydate_to_proto_date(date.fromisoformat(end))
            except Exception:
                console.print(f"[red]Invalid end date: [yellow]{end}")
                return

        task = progress.add_task(f"{session.backtest.name}")
        for _, period in session.run_execution(
            start_date,
            end_date,
            [s for s in session.backtest.symbols],
        ):
            progress.update(task, total=period.num_total_periods, completed=period.num_current_period)
        log.info(f"Execution completed for {session.backtest.name}")


"""
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
"""
