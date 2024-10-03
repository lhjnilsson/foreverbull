from datetime import datetime

import typer
from foreverbull import broker
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.pb_utils import to_proto_timestamp
from rich.console import Console
from rich.table import Table
from typing_extensions import Annotated

name_option = Annotated[str, typer.Option(help="name of the backtest")]
session_argument = Annotated[str, typer.Argument(help="session id of the backtest")]

backtest = typer.Typer()

std = Console()
std_err = Console(stderr=True)


@backtest.command()
def list():
    backtests = broker.backtest.list()

    table = Table(title="Backtests")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")

    for backtest in backtests:
        table.add_row(
            backtest.name,
            (
                backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status)
                if backtest.statuses
                else "Unknown"
            ),
            backtest.start_date.ToDatetime().isoformat() if backtest.start_date else "",
            backtest.end_date.ToDatetime().isoformat() if backtest.end_date else "",
            ",".join(backtest.symbols),
            backtest.benchmark,
        )
    std.print(table)


@backtest.command()
def create(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
    start: Annotated[datetime, typer.Option(help="start time of the backtest")],
    end: Annotated[datetime, typer.Option(help="end time of the backtest")],
    symbols: Annotated[
        str, typer.Option(help="comma separated list of symbols to use")
    ],
    benchmark: (
        Annotated[str, typer.Option(help="symbol of benchmark to use")] | None
    ) = None,
):
    backtest = backtest_pb2.Backtest(
        name=name,
        start_date=to_proto_timestamp(start),
        end_date=to_proto_timestamp(end),
        symbols=(
            [symbol.strip().upper() for symbol in symbols.split(",")] if symbols else []
        ),
        benchmark=benchmark,
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
        (
            backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status)
            if backtest.statuses
            else "Unknown"
        ),
        backtest.start_date.ToDatetime().isoformat() if backtest.start_date else "",
        backtest.end_date.ToDatetime().isoformat() if backtest.end_date else "",
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    std.print(table)


@backtest.command()
def get(
    backtest_name: Annotated[str, typer.Argument(help="name of the backtest")],
):
    backtest = broker.backtest.get(backtest_name)
    table = Table(title="Backtest")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")
    table.add_row(
        backtest.name,
        (
            backtest_pb2.Backtest.Status.Status.Name(backtest.statuses[0].status)
            if backtest.statuses
            else "Unknown"
        ),
        backtest.start_date.ToDatetime().isoformat() if backtest.start_date else "",
        backtest.end_date.ToDatetime().isoformat() if backtest.end_date else "",
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    std.print(table)


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
