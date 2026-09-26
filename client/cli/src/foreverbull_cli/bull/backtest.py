import logging

from datetime import date
from enum import Enum
from pathlib import Path
from typing import Annotated

import typer
import yaml

from rich.prompt import Confirm
from rich.prompt import Prompt
from rich.table import Table

from foreverbull import broker
from foreverbull import models
from foreverbull.foreverbull import Foreverbull
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.bull.etf_catalog import etf_by_name
from foreverbull_cli.bull.etf_catalog import etf_catalog
from foreverbull_cli.bull.etf_catalog import known_etf_names
from foreverbull_cli.output import LiveProgress
from foreverbull_cli.output import console
from foreverbull_cli.output import execution_link
from foreverbull_cli.output import format_date_field
from foreverbull_cli.output import format_optional_float
from foreverbull_cli.output import format_proto_date


backtest = typer.Typer(help="Create and inspect backtests. `etfs` lists known iShares funds.")
log = logging.getLogger().getChild(__name__)


def _ask_name() -> str:
    while True:
        name = Prompt.ask("What name do you want to give the backtest?").strip()
        if name:
            return name
        console.print("Enter a name for the backtest.")


def _ask_date(prompt: str) -> date:
    while True:
        raw = Prompt.ask(prompt).strip()
        try:
            return date.fromisoformat(raw)
        except ValueError:
            console.print(f"{raw!r} is not a date. Use YYYY-MM-DD, for example 2024-01-31.")


def _ask_etf() -> str | None:
    names = [fund.name for fund in etf_catalog()]
    if not names:
        console.print("No ETFs are configured.")
        return None
    default = "iShares S&P 100" if "iShares S&P 100" in names else names[0]
    prompt = Prompt(
        "What ETF do you want to base the backtest on?",
        console=console,
        choices=names,
    )
    raw = Prompt.get_input(console, prompt.make_prompt(default), password=False).strip()
    if raw == "":
        raw = default
    if raw not in names:
        console.print(f"Unknown ETF {raw!r}. Known funds: {known_etf_names()}.")
        return None
    return raw


def _ask_backtest_dates(etf: str) -> tuple[date, date]:
    fund = etf_by_name(etf)
    if fund is None:
        console.print(f"Unknown ETF {etf!r}. Known funds: {known_etf_names()}.")
        raise typer.Exit(code=1)
    inception = fund.inception
    today = date.today()
    while True:
        start = _ask_date("What is the start date of the backtest? (YYYY-MM-DD)")
        if start < inception:
            console.print(f"{etf} started on {inception.isoformat()}. Choose a start date on or after that day.")
            continue
        if start > today:
            console.print(f"{start.isoformat()} is after today ({today.isoformat()}).")
            continue
        while True:
            end = _ask_date("What is the end date of the backtest? (YYYY-MM-DD)")
            if end < start:
                console.print(f"End date {end.isoformat()} is before start date {start.isoformat()}.")
                continue
            if end > today:
                console.print(f"End date {end.isoformat()} is after today ({today.isoformat()}).")
                continue
            return start, end


def _enabled_symbols(entity: backtest_pb2.Backtest) -> str:
    symbols: list[str] = []
    seen: set[str] = set()
    for current in entity.periods:
        for asset in current.assets:
            if asset.enabled and asset.symbol not in seen:
                seen.add(asset.symbol)
                symbols.append(asset.symbol)
    return ",".join(symbols)


def _backtest_range(entity: backtest_pb2.Backtest) -> tuple[str, str]:
    if not entity.periods:
        return "", ""
    start = format_date_field(entity.periods[0], "start_date")
    end = format_date_field(entity.periods[-1], "end_date")
    return start, end


def _print_execution_link(execution_id: str) -> None:
    if execution_id:
        console.print(f"Link: {execution_link(execution_id)}")


def _print_executions(executions: list[execution_pb2.Execution], include_backtest: bool = False) -> None:
    table = Table(title="executions")
    table.add_column("ID")
    if include_backtest:
        table.add_column("Backtest")
        table.add_column("Session")
    table.add_column("Start")
    table.add_column("End")
    for execution in executions:
        row = [execution.id]
        if include_backtest:
            row.extend([execution.backtest, execution.session])
        row.extend(
            [
                format_date_field(execution, "start_date"),
                format_date_field(execution, "end_date"),
            ]
        )
        table.add_row(*row)
    console.print(table)
    for execution in executions:
        _print_execution_link(execution.id)


@backtest.command("list")
def list_backtests():
    table = Table(title="Backtests")
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Benchmark")
    for entity in broker.backtest.list_backtests():
        if not len(entity.periods):
            console.print(f"Backtest {entity.name} has no periods")
            continue
        start, end = _backtest_range(entity)
        table.add_row(entity.name, start, end, entity.benchmark)
    console.print(table)


@backtest.command("etfs")
def list_etfs() -> None:
    """List known iShares ETFs (name, benchmark, and inception)."""
    table = Table(title="ETFs")
    table.add_column("Name")
    table.add_column("Benchmark")
    table.add_column("Inception")
    for fund in etf_catalog():
        table.add_row(fund.name, fund.benchmark, fund.inception.isoformat())
    console.print(table)


@backtest.command()
def create(
    file: Annotated[
        Path | None,
        typer.Option(help="path to the backtest file[YAML]. Omit to choose an ETF from `fbull backtest etfs`"),
    ] = None,
):
    """Create a backtest from a YAML file or a known iShares ETF.

    Without --file, ETF prompt choices come from `fbull backtest etfs`.
    An unknown name stops before any download.
    """
    entity: (
        backtest_service_pb2.CreateBacktestRequest.CreateFromETF
        | backtest_service_pb2.CreateBacktestRequest.CreateFromBacktest
    )
    if file:
        if not file.exists():
            raise FileNotFoundError(f"File {file} not found")
        with open(file, "r") as f:
            data = yaml.safe_load(f)
        entity = backtest_service_pb2.CreateBacktestRequest.CreateFromBacktest(
            backtest=backtest_pb2.Backtest(
                name=data["name"],
                benchmark=data["benchmark"],
                periods=[
                    backtest_pb2.Backtest.Period(
                        start_date=from_pydate_to_proto_date(date.fromisoformat(period["start_date"])),
                        end_date=from_pydate_to_proto_date(date.fromisoformat(period["end_date"])),
                        assets=[
                            backtest_pb2.Backtest.Period.Asset(
                                symbol=asset["symbol"],
                                enabled=asset["enabled"],
                            )
                            for asset in period["assets"]
                        ],
                    )
                    for period in data["periods"]
                ],
            )
        )
    else:
        name = _ask_name()
        base_etf = _ask_etf()
        if base_etf is None:
            return
        start, end = _ask_backtest_dates(base_etf)
        entity = backtest_service_pb2.CreateBacktestRequest.CreateFromETF(
            name=name,
            base_etf=base_etf,
            start_date=from_pydate_to_proto_date(start),
            end_date=from_pydate_to_proto_date(end),
        )

    created: backtest_pb2.Backtest | None = None
    with LiveProgress() as progress:
        rsp = broker.backtest.create(
            entity=entity,
        )
        for backtest_rsp in rsp:
            progress.update_from_message(backtest_rsp)
            if backtest_rsp.STORING.state == common_pb2.State.STATE_COMPLETED:
                created = backtest_rsp.backtest

    if created is None:
        return

    start, end = _backtest_range(created)
    table = Table(title="Created Backtest", expand=True)
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")
    table.add_row(
        created.name,
        start,
        end,
        _enabled_symbols(created),
        created.benchmark,
    )
    console.print(table)


class OutputFormat(str, Enum):
    DEFAULT = "default"
    YAML = "yaml"


@backtest.command()
def get(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
    output: Annotated[OutputFormat, typer.Option(help="output file")] = OutputFormat.DEFAULT,
):
    entity = broker.backtest.get(name)
    if output == OutputFormat.YAML:
        backtest_dict = {
            "name": entity.name,
            "benchmark": entity.benchmark,
            "periods": [],
        }
        for period in entity.periods:
            backtest_dict["periods"].append(
                {
                    "start_date": pb_utils.from_proto_date_to_pydate(period.start_date).isoformat(),
                    "end_date": pb_utils.from_proto_date_to_pydate(period.end_date).isoformat(),
                    "assets": [
                        {
                            "symbol": asset.symbol,
                            "enabled": asset.enabled,
                        }
                        for asset in period.assets
                    ],
                }
            )
        print(yaml.dump(backtest_dict))
        return

    start, end = _backtest_range(entity)
    backtest_table = Table(title="Backtest", expand=True)
    backtest_table.add_column("Name")
    backtest_table.add_column("Start")
    backtest_table.add_column("End")
    backtest_table.add_column("Symbols", overflow="fold")
    backtest_table.add_column("Benchmark")
    backtest_table.add_row(
        entity.name,
        start,
        end,
        _enabled_symbols(entity),
        entity.benchmark,
    )
    periods_table = Table(title="periods", expand=True, show_lines=True)
    periods_table.add_column("Start")
    periods_table.add_column("End")
    periods_table.add_column("Enabled Symbols", overflow="fold")
    periods_table.add_column("Disabled Symbols", overflow="fold")
    for period in entity.periods:
        periods_table.add_row(
            format_date_field(period, "start_date"),
            format_date_field(period, "end_date"),
            ",".join([asset.symbol for asset in period.assets if asset.enabled]),
            ",".join([asset.symbol for asset in period.assets if not asset.enabled]),
        )
    executions = broker.backtest.list_executions(name, None)

    console.print(backtest_table)
    console.print(periods_table)
    _print_executions(executions)


@backtest.command()
def delete(
    names: Annotated[list[str], typer.Argument(help="name of the backtest, can be multiple")],
    force: Annotated[bool, typer.Option(help="force delete backtest without confirmation")] = False,
):
    for name in names:
        _ = broker.backtest.get(name)
        if not force:
            shall_delete = Confirm.ask(f"Are you sure you want to delete backtest {name}?", default=False)
            if not shall_delete:
                return

        broker.backtest.delete(name)


def _parse_run_dates(
    entity: backtest_pb2.Backtest,
    start: str | None,
    end: str | None,
) -> tuple[common_pb2.Date, common_pb2.Date] | None:
    if not entity.periods:
        console.print(f"[red]Backtest {entity.name} has no periods")
        return None

    start_date = entity.periods[0].start_date
    if start:
        try:
            start_date = from_pydate_to_proto_date(date.fromisoformat(start))
        except ValueError:
            console.print(f"[red]Invalid start date: [yellow]{start}")
            return None

    end_date = entity.periods[-1].end_date
    if end:
        try:
            end_date = from_pydate_to_proto_date(date.fromisoformat(end))
        except ValueError:
            console.print(f"[red]Invalid end date: [yellow]{end}")
            return None

    return start_date, end_date


@backtest.command()
def run(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
    file_path: Annotated[str, typer.Argument(help="path to the strategy file")],
    start: Annotated[str | None, typer.Option(help="start date of the backtest")] = None,
    end: Annotated[str | None, typer.Option(help="end date of the backtest")] = None,
    dry: Annotated[bool, typer.Option("--dry", help="print the execution plan without running")] = False,
    yes: Annotated[bool, typer.Option("--yes", help="skip confirmation prompt")] = False,
):
    entity = broker.backtest.get(name)
    dates = _parse_run_dates(entity, start, end)
    if dates is None:
        return
    start_date, end_date = dates
    start_iso = format_proto_date(start_date)
    end_iso = format_proto_date(end_date)

    if dry:
        table = Table(title="Dry Run")
        table.add_column("Name")
        table.add_column("File")
        table.add_column("Start")
        table.add_column("End")
        table.add_row(entity.name, file_path, start_iso, end_iso)
        console.print(table)
        return

    if not yes:
        shall_run = Confirm.ask(
            f"Run backtest {entity.name} from {start_iso} to {end_iso}?",
            default=False,
        )
        if not shall_run:
            return

    execution_id = None
    with LiveProgress() as progress:
        has_run = False
        for msg in broker.backtest.create_session(name):
            progress.update_from_message(msg)
            if msg.session.port and not has_run:
                has_run = True
                with Foreverbull(models.Algorithm.from_file_path(file_path)).backtest_session(
                    entity, msg.session
                ) as session:
                    with progress.bar_progress("execution") as bar_progress:
                        for execution_id, message in session.run_execution(
                            start_date,
                            end_date,
                        ):
                            bar_progress(message.num_current_period, message.num_total_periods)
                    log.info(f"Execution completed for {session.backtest.name}")

    table = Table(title="Execution")
    table.add_column("ID")
    table.add_column("Backtest")
    table.add_column("Start")
    table.add_column("End")
    table.add_row(
        execution_id or "",
        entity.name,
        start_iso,
        end_iso,
    )
    console.print(table)
    _print_execution_link(execution_id or "")


@backtest.command()
def executions(
    backtest: Annotated[str | None, typer.Option(help="name of the backtest")] = None,
    session: Annotated[str | None, typer.Option(help="id of the session")] = None,
):
    rows = broker.backtest.list_executions(backtest, session)
    _print_executions(rows, include_backtest=True)


@backtest.command()
def execution(
    id: Annotated[str, typer.Argument(help="id of the execution")],
):
    entity, periods = broker.backtest.get_execution(id)

    summary = Table(title="Execution")
    summary.add_column("ID")
    summary.add_column("Backtest")
    summary.add_column("Session")
    summary.add_column("Start")
    summary.add_column("End")
    summary.add_row(
        entity.id,
        entity.backtest,
        entity.session,
        format_date_field(entity, "start_date"),
        format_date_field(entity, "end_date"),
    )
    console.print(summary)
    _print_execution_link(entity.id)

    table = Table(title="periods")
    table.add_column("Date", no_wrap=True)
    table.add_column("Portfolio")
    table.add_column("Alpha")
    table.add_column("Beta")
    table.add_column("Sharpe")
    table.add_column("Sortino")

    for period in periods:
        table.add_row(
            format_proto_date(period.date),
            f"{period.portfolio_value:.5f}",
            format_optional_float(period, "alpha"),
            format_optional_float(period, "beta"),
            format_optional_float(period, "sharpe"),
            format_optional_float(period, "sortino"),
        )
    console.print(table)
