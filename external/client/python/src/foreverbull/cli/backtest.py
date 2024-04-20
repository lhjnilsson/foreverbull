import signal
import socket
import time
from datetime import datetime

import typer
from rich.console import Console
from rich.progress import Progress
from rich.table import Table
from typing_extensions import Annotated

from foreverbull import Foreverbull, broker, entity

name_option = Annotated[
    str,
    typer.Option(help="name of the backtest"),
]
session_argument = Annotated[
    str,
    typer.Argument(help="session id of the backtest"),
]

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
            (backtest.statuses[0].status.value if backtest.statuses else "Unknown"),
            backtest.start.isoformat(),
            backtest.end.isoformat(),
            ",".join(backtest.symbols),
            backtest.benchmark,
        )
    std.print(table)


@backtest.command()
def create(
    name: Annotated[
        str,
        typer.Argument(help="name of the backtest"),
    ],
    start: Annotated[
        datetime,
        typer.Option(help="start time of the backtest"),
    ],
    end: Annotated[
        datetime,
        typer.Option(help="end time of the backtest"),
    ],
    symbols: Annotated[
        str,
        typer.Option(help="comma separated list of symbols to use"),
    ],
    service: Annotated[
        str,
        typer.Option(help="worker service to use"),
    ] = None,
    benchmark: Annotated[
        str,
        typer.Option(help="symbol of benchmark to use"),
    ] = None,
):
    backtest = entity.backtest.Backtest(
        name=name,
        service=service,
        start=start,
        end=end,
        symbols=[symbol.strip().upper() for symbol in symbols.split(",")],
        benchmark=benchmark,
    )
    with Progress() as progress:
        task = progress.add_task(
            "Created",
            total=2,
        )
        backtest = broker.backtest.create(backtest)
        previous_status = backtest.statuses[0].status
        while not progress.finished:
            time.sleep(0.5)
            backtest = broker.backtest.get(name)
            status = backtest.statuses[0].status
            if previous_status and previous_status != status:
                match status:
                    case entity.backtest.BacktestStatusType.INGESTING:
                        progress.advance(task)
                        progress.update(
                            task,
                            description="Ingesting",
                        )
                    case entity.backtest.BacktestStatusType.READY:
                        progress.advance(task)
                        progress.update(
                            task,
                            description="Ready",
                        )
                    case entity.backtest.BacktestStatusType.ERROR:
                        std_err.log(f"[red]Error while creating backtest: {backtest.statuses[0].error}")
                        exit(1)
                previous_status = status

    table = Table(title="Created Backtest")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")

    table.add_row(
        backtest.name,
        (backtest.statuses[0].status.value if backtest.statuses else "Unknown"),
        backtest.start.isoformat(),
        backtest.end.isoformat(),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    std.print(table)


@backtest.command()
def get(
    backtest_name: Annotated[
        str,
        typer.Argument(help="name of the backtest"),
    ],
):
    backtest = broker.backtest.get(backtest_name)
    sessions = broker.backtest.list_sessions(backtest_name)
    table = Table(title="Backtest")
    table.add_column("Name")
    table.add_column("Status")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols")
    table.add_column("Benchmark")
    table.add_row(
        backtest.name,
        (backtest.statuses[0].status.value if backtest.statuses else "Unknown"),
        backtest.start.isoformat(),
        backtest.end.isoformat(),
        ",".join(backtest.symbols),
        backtest.benchmark,
    )
    std.print(table)

    table = Table(title="Sessions")
    table.add_column("Id")
    table.add_column("Status")
    table.add_column("Date")
    table.add_column("Executions")
    for session in sessions:
        table.add_row(
            session.id,
            (session.statuses[0].status.value if session.statuses else "Unknown"),
            (session.statuses[0].occurred_at.isoformat() if session.statuses else "Unknown"),
            str(session.executions),
        )
    std.print(table)


@backtest.command()
def run(
    file_path: Annotated[
        str,
        typer.Argument(help="name of the file to use"),
    ],
    backtest_name: Annotated[
        str,
        typer.Option(help="name of the backtest"),
    ] = None,
):
    def show_progress(
        session: entity.backtest.Session,
    ):
        with Progress() as progress:
            task = progress.add_task(
                "Starting",
                total=2,
            )
            previous_status = session.statuses[0].status
            while not progress.finished:
                time.sleep(0.5)
                session = broker.backtest.get_session(session.id)
                status = session.statuses[0].status
                if previous_status and previous_status != status:
                    match status:
                        case entity.backtest.SessionStatusType.RUNNING:
                            progress.advance(task)
                            progress.update(
                                task,
                                description="Running",
                            )
                        case entity.backtest.SessionStatusType.COMPLETED:
                            progress.advance(task)
                            progress.update(
                                task,
                                description="Completed",
                            )
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
            (session.statuses[0].status.value if session.statuses else "Unknown"),
            (session.statuses[0].occurred_at.isoformat() if session.statuses else "Unknown"),
            str(session.executions),
        )
        std.print(table)

    if backtest_name:
        session = broker.backtest.run(
            backtest_name,
            manual=(True if file_path else False),
        )
        foreverbull = Foreverbull(
            session,
            file_path=file_path,
        )
        with foreverbull as fb:
            show_progress(fb.session)
        return

    # TODO: Actually fetch the correct session with session_id from env
    # To be set from container in foreverbull broker
    session = entity.backtest.Session(
        id="dummy123",
        backtest="dummy",
        executions=0,
    )
    foreverbull = Foreverbull(
        session,
        file_path=file_path,
    )
    with foreverbull as fb:
        broker.service.update_instance(
            socket.gethostname(),
            fb.socket_config,
        )
        signal.signal(
            signal.SIGINT,
            lambda x, y: foreverbull._stop_event.set(),
        )
        signal.signal(
            signal.SIGTERM,
            lambda x, y: foreverbull._stop_event.set(),
        )
        fb.join()
        broker.service.update_instance(
            socket.gethostname(),
            None,
        )


@backtest.command()
def executions(
    session_id: session_argument,
):
    executions = broker.backtest.list_executions(session_id)
    table = Table(title="Executions")
    table.add_column("Id")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Status")

    for execution in executions:
        table.add_row(
            execution.id,
            execution.start.isoformat(),
            execution.end.isoformat(),
            (execution.statuses[0].status.value if execution.statuses else "Unknown"),
        )
    std.print(table)
