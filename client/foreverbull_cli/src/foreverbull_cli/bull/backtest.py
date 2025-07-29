import logging

from datetime import date
from datetime import datetime

import typer

from rich.table import Table
from typing_extensions import Annotated

from foreverbull import broker
from foreverbull import models
from foreverbull.foreverbull import Foreverbull
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.pb_utils import from_proto_date_to_pydate
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.output import LiveProgress
from foreverbull_cli.output import console


backtest = typer.Typer()
log = logging.getLogger().getChild(__name__)


@backtest.command()
def list():
    table = Table(title="Backtests")
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Benchmark")
    for backtest in broker.backtest.list_backtests():
        table.add_row(
            backtest.name,
            (from_proto_date_to_pydate(backtest.periods[0].start_date).isoformat()),
            (
                from_proto_date_to_pydate(backtest.periods[-1].end_date).isoformat()
                if backtest.periods[-1].HasField("end_date")
                else None
            ),
            backtest.benchmark,
        )
    console.print(table)


@backtest.command()
def create(
    name: Annotated[str, typer.Option(help="name of the backtest")],
    base_etf: Annotated[str, typer.Option(help="name of the etf to base the backtest on")],
    start_date: Annotated[datetime, typer.Option(help="start date of the backtest")],
    end_date: Annotated[datetime, typer.Option(help="end date of the backtest")],
):
    backtest_rsp = backtest_service_pb2.CreateBacktestResponse()
    with LiveProgress() as progress:
        rsp = broker.backtest.create(
            name=name,
            base_etf=base_etf,
            start_date=from_pydate_to_proto_date(start_date),
            end_date=from_pydate_to_proto_date(end_date),
        )
        for backtest_rsp in rsp:
            progress.update_from_message(backtest_rsp)

    if not backtest_rsp.STORING.state == common_pb2.State.STATE_COMPLETED:
        return

    table = Table(title="Created Backtest", expand=True)
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")
    table.add_row(
        backtest_rsp.backtest.name,
        (from_proto_date_to_pydate(backtest_rsp.backtest.start_date).isoformat() if False else ""),
        (from_proto_date_to_pydate(backtest_rsp.backtest.end_date).isoformat() if False else ""),
        "",
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
        from_proto_date_to_pydate(backtest.periods[0].start_date).isoformat(),
        (
            from_proto_date_to_pydate(backtest.periods[-1].end_date).isoformat()
            if backtest.periods[-1].HasField("end_date")
            else None
        ),
        ",".join([]),
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
    backtest = broker.backtest.get(name)
    with LiveProgress() as progress:
        has_run = False
        for msg in broker.backtest.create_session(name):
            progress.update_from_message(msg)
            if msg.session.port and not has_run:
                has_run = True
                with Foreverbull(models.Algorithm.from_file_path(file_path)).backtest_session(
                    backtest, msg.session
                ) as session:
                    start_date = session.backtest.periods[0].start_date
                    if start:
                        try:
                            start_date = from_pydate_to_proto_date(date.fromisoformat(start))
                        except Exception:
                            console.print(f"[red]Invalid start date: [yellow]{start}")
                            return

                    end_date = session.backtest.periods[-1].end_date
                    if end:
                        try:
                            end_date = from_pydate_to_proto_date(date.fromisoformat(end))
                        except Exception:
                            console.print(f"[red]Invalid end date: [yellow]{end}")
                            return

                    with progress.bar_progress("execution") as bar_progress:
                        for _, period in session.run_execution(
                            start_date,
                            end_date,
                        ):
                            bar_progress(period.num_current_period, period.num_total_periods)
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
