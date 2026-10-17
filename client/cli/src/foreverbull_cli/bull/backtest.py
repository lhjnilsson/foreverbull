import logging
import re

from datetime import date
from enum import Enum
from pathlib import Path
from typing import Annotated

import grpc
import typer
import yaml

from rich.prompt import Confirm
from rich.prompt import Prompt
from rich.table import Table

from foreverbull import broker
from foreverbull import models
from foreverbull.foreverbull import Foreverbull
from foreverbull.models.finance import DEFAULT_PROVIDER
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
_ISO_DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_DEFAULT_PROVIDER = DEFAULT_PROVIDER
_PRICE_PROVIDERS = ("yahoo", "stooq", "alpaca")
_PRICE_PROVIDER_SET = frozenset(_PRICE_PROVIDERS)


def _normalize_provider(raw: object) -> str | None:
    provider = "" if raw is None else str(raw).strip().lower()
    if provider == "":
        provider = _DEFAULT_PROVIDER
    if provider not in _PRICE_PROVIDER_SET:
        console.print(f"[red]Unknown price provider {provider!r}. Valid providers: {', '.join(_PRICE_PROVIDERS)}.")
        return None
    return provider


def _choose_provider(flag_provider: str | None, raw: object, file_name: str) -> tuple[str | None, str]:
    """Return the provider and where it came from: the flag, then the YAML file, then the default."""
    if flag_provider is not None:
        return flag_provider, "from --provider"
    if raw is None or str(raw).strip() == "":
        return _DEFAULT_PROVIDER, "default"
    return _normalize_provider(raw), f"from {file_name or 'backtest.yaml'}"


def _fail(message: str) -> None:
    """Print one ✖ line and exit 1, like the fbenv host checks."""
    console.print(
        f"✖ {message.strip() or 'Creating the backtest failed.'}", markup=False, highlight=False, soft_wrap=True
    )
    raise typer.Exit(code=1)


def _rpc_details(exc: grpc.RpcError) -> str:
    details = getattr(exc, "details", None)
    return str(details()) if callable(details) else str(exc)


def _entity_name(
    entity: backtest_service_pb2.CreateBacktestRequest.CreateFromETF
    | backtest_service_pb2.CreateBacktestRequest.CreateFromBacktest,
) -> str:
    if isinstance(entity, backtest_service_pb2.CreateBacktestRequest.CreateFromBacktest):
        return entity.backtest.name
    return entity.name


def _discard_partial_backtest(name: str, failure: str) -> str:
    """Delete a backtest this create stored before its stream broke.

    Only called after this stream reported CREATING as completed, so the
    record is ours and not an older backtest with the same name. Bars are
    stored per provider and symbol, so nothing else is removed.
    """
    reason = failure.strip().rstrip(".") or "Creating the backtest failed"
    try:
        broker.backtest.delete(name)
    except grpc.RpcError as exc:
        code = getattr(exc, "code", None)
        if not callable(code) or code() != grpc.StatusCode.NOT_FOUND:
            return (
                f"{reason}. The backtest {name} could not be removed; "
                f"delete it with `fbull backtest delete {name}` before you retry."
            )
    return f"{reason}. Nothing was changed."


def _optional_profile(raw: object) -> str:
    if raw is None:
        return ""
    return str(raw).strip()


def _shown_provider(entity: backtest_pb2.Backtest) -> str:
    provider = entity.provider.strip().lower()
    return provider or _DEFAULT_PROVIDER


def _read_line(prompt: Prompt, default: str | None = None) -> str | None:
    shown = default if default is not None else ...
    try:
        line = Prompt.get_input(console, prompt.make_prompt(shown), password=False)
    except EOFError:
        return None
    return line.strip()


def _ask_name() -> str | None:
    prompt = Prompt("What name do you want to give the backtest?", console=console)
    while True:
        name = _read_line(prompt)
        if name is None:
            return None
        if name:
            return name
        console.print("Enter a name for the backtest.")


def _parse_iso_date(raw: str) -> date | None:
    if _ISO_DATE.fullmatch(raw) is None:
        return None
    try:
        return date.fromisoformat(raw)
    except ValueError:
        return None


def _ask_date(prompt_text: str) -> date | None:
    prompt = Prompt(prompt_text, console=console)
    while True:
        raw = _read_line(prompt)
        if raw is None:
            return None
        parsed = _parse_iso_date(raw)
        if parsed is not None:
            return parsed
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
    while True:
        raw = _read_line(prompt, default)
        if raw is None:
            return None
        if raw == "":
            raw = default
        if raw not in names:
            console.print(f"Unknown ETF {raw!r}. Known funds: {known_etf_names()}.")
            continue
        return raw


def _ask_backtest_dates(etf: str) -> tuple[date, date] | None:
    fund = etf_by_name(etf)
    if fund is None:
        console.print(f"Unknown ETF {etf!r}. Known funds: {known_etf_names()}.")
        return None
    inception = fund.inception
    today = date.today()
    while True:
        start = _ask_date("What is the start date of the backtest? (YYYY-MM-DD)")
        if start is None:
            return None
        if start < inception:
            console.print(f"{etf} started on {inception.isoformat()}. Choose a start date on or after that day.")
            continue
        if start > today:
            console.print(f"{start.isoformat()} is after today ({today.isoformat()}).")
            continue
        while True:
            end = _ask_date("What is the end date of the backtest? (YYYY-MM-DD)")
            if end is None:
                return None
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
    table.add_column("Provider")
    for entity in broker.backtest.list_backtests():
        if not len(entity.periods):
            console.print(f"Backtest {entity.name} has no periods")
            continue
        start, end = _backtest_range(entity)
        table.add_row(entity.name, start, end, entity.benchmark, _shown_provider(entity))
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
    provider: Annotated[
        str | None,
        typer.Option(
            "--provider",
            show_default=False,
            help="Price source: yahoo, stooq or alpaca. Overrides `provider` in the YAML file. Default: yahoo.",
        ),
    ] = None,
):
    """Create a backtest from a YAML file or a known iShares ETF.

    Without --file, ETF prompt choices come from `fbull backtest etfs`.
    A bad name, fund, or date is re-asked before any download.
    Optional YAML fields `provider` and `profile` are sent on the create request.
    `--provider` overrides the file. An omitted provider is sent as yahoo.
    """
    flag_provider: str | None = None
    if provider is not None:
        flag_provider = _normalize_provider(provider)
        if flag_provider is None:
            raise typer.Exit(code=1)

    entity: (
        backtest_service_pb2.CreateBacktestRequest.CreateFromETF
        | backtest_service_pb2.CreateBacktestRequest.CreateFromBacktest
    )
    if file:
        if not file.exists():
            raise FileNotFoundError(f"File {file} not found")
        with open(file, "r") as f:
            data = yaml.safe_load(f)
        chosen, source = _choose_provider(flag_provider, data.get("provider"), file.name)
        if chosen is None:
            raise typer.Exit(code=1)
        entity = backtest_service_pb2.CreateBacktestRequest.CreateFromBacktest(
            backtest=backtest_pb2.Backtest(
                name=data["name"],
                benchmark=data["benchmark"],
                provider=chosen,
                profile=_optional_profile(data.get("profile")),
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
        if name is None:
            return
        base_etf = _ask_etf()
        if base_etf is None:
            return
        dates = _ask_backtest_dates(base_etf)
        if dates is None:
            return
        start, end = dates
        chosen, source = _choose_provider(flag_provider, None, "")
        entity = backtest_service_pb2.CreateBacktestRequest.CreateFromETF(
            name=name,
            base_etf=base_etf,
            start_date=from_pydate_to_proto_date(start),
            end_date=from_pydate_to_proto_date(end),
            provider=chosen,
        )

    console.print(f"Using provider: {chosen} ({source})", markup=False)

    created: backtest_pb2.Backtest | None = None
    stored = False
    failure: str | None = None
    with LiveProgress() as progress:
        try:
            for backtest_rsp in broker.backtest.create(entity=entity):
                progress.update_from_message(backtest_rsp)
                if backtest_rsp.CREATING.state == common_pb2.State.STATE_COMPLETED:
                    stored = True
                if backtest_rsp.STORING.state == common_pb2.State.STATE_COMPLETED:
                    created = backtest_rsp.backtest
        except grpc.RpcError as exc:
            failure = _rpc_details(exc)
            if stored and created is None:
                failure = _discard_partial_backtest(_entity_name(entity), failure)
        failure = failure or progress.failure
        if failure is not None:
            # The failed step's message is the whole error, so the progress
            # rows are cleared rather than left beside it.
            progress.transient = True

    if failure is not None:
        _fail(failure)

    if created is None:
        return

    start, end = _backtest_range(created)
    table = Table(title="Created Backtest", expand=True)
    table.add_column("Name")
    table.add_column("Start")
    table.add_column("End")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Benchmark")
    table.add_column("Provider")
    table.add_row(
        created.name,
        start,
        end,
        _enabled_symbols(created),
        created.benchmark,
        _shown_provider(created),
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
            "provider": _shown_provider(entity),
            "periods": [],
        }
        profile = entity.profile.strip()
        if profile:
            backtest_dict["profile"] = profile
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
    backtest_table.add_column("Provider")
    backtest_table.add_row(
        entity.name,
        start,
        end,
        _enabled_symbols(entity),
        entity.benchmark,
        _shown_provider(entity),
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
