import logging
import os
import re

from datetime import date
from enum import Enum
from pathlib import Path
from typing import Annotated
from typing import NoReturn

import grpc
import typer
import yaml

from rich.prompt import Confirm
from rich.prompt import Prompt
from rich.table import Table

from foreverbull import broker
from foreverbull import models
from foreverbull.broker.utils import redact_text
from foreverbull.foreverbull import Foreverbull
from foreverbull.models.finance import DEFAULT_PROVIDER
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.pb_utils import from_pydate_to_proto_date
from foreverbull_cli.bull.etf_catalog import etf_by_name
from foreverbull_cli.bull.etf_catalog import etf_catalog
from foreverbull_cli.bull.etf_catalog import known_etf_names
from foreverbull_cli.bull.summary import distinguishing_execution_ids
from foreverbull_cli.bull.summary import print_run_summary
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
_FULL_EXECUTION_ID_LENGTH = 36  # a UUID with hyphens
_AMBIGUOUS_MATCHES_SHOWN = 5
# The broker appends the redacted cause of a failed step after this marker.
_DETAILS_MARKER = "\nDetails: "
_VERBOSE_LEVELS = frozenset({"WARNING", "INFO", "DEBUG"})


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


def _verbose() -> bool:
    """True with `fbull --verbose`, which lowers LOGGING_LEVEL below ERROR."""
    level = os.environ.get("LOGGING_LEVEL", "ERROR").strip().upper()
    return level in _VERBOSE_LEVELS


def _fail(message: str) -> NoReturn:
    """Print one ✖ line and exit 1, like the fbenv host checks.

    The server puts the underlying exception after a details marker. It is
    already redacted there, and shown only with --verbose.
    """
    # A server older than this client may still quote a database URL.
    shown, marker, details = redact_text(message).partition(_DETAILS_MARKER)
    console.print(
        f"✖ {shown.strip() or 'Creating the backtest failed.'}", markup=False, highlight=False, soft_wrap=True
    )
    if marker and details.strip() and _verbose():
        console.print(f"Details: {details.strip()}", markup=False, highlight=False, soft_wrap=True)
    raise typer.Exit(code=1)


def _rpc_details(exc: grpc.RpcError) -> str:
    details = getattr(exc, "details", None)
    return str(details()) if callable(details) else str(exc)


def _entity_name(
    entity: backtest_service_pb.CreateBacktestRequest.CreateFromETF
    | backtest_service_pb.CreateBacktestRequest.CreateFromBacktest,
) -> str:
    if isinstance(entity, backtest_service_pb.CreateBacktestRequest.CreateFromBacktest):
        backtest = entity.backtest
        return "" if backtest is None else backtest.name
    return entity.name


def _discard_partial_backtest(name: str, failure: str) -> str:
    """Delete a backtest this create stored before its stream broke.

    Only called after this stream reported CREATING as completed, so the
    record is ours and not an older backtest with the same name. Bars are
    stored per provider and symbol, so nothing else is removed.
    """
    reason = failure.strip().rstrip(".")
    try:
        broker.backtest.delete(name)
    except grpc.RpcError as exc:
        code = getattr(exc, "code", None)
        if not callable(code) or code() != grpc.StatusCode.NOT_FOUND:
            return (
                f"{reason or 'Creating the backtest failed'}. The backtest {name} could not be removed; "
                f"delete it with `fbull backtest delete {name}` before you retry."
            )
    if not reason:
        return "Creating the backtest failed. Nothing was changed. Run the same create again."
    detail = reason[0].lower() + reason[1:]
    return f"Creating the backtest failed: {detail}. Nothing was changed. Run the same create again."


def _optional_profile(raw: object) -> str:
    if raw is None:
        return ""
    return str(raw).strip()


def _shown_provider(entity: backtest_pb.Backtest) -> str:
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


def _enabled_symbols(entity: backtest_pb.Backtest) -> str:
    symbols: list[str] = []
    seen: set[str] = set()
    for current in entity.periods:
        for asset in current.assets:
            if asset.enabled and asset.symbol not in seen:
                seen.add(asset.symbol)
                symbols.append(asset.symbol)
    return ",".join(symbols)


def _backtest_range(entity: backtest_pb.Backtest) -> tuple[str, str]:
    if not entity.periods:
        return "", ""
    start = format_date_field(entity.periods[0], "start_date")
    end = format_date_field(entity.periods[-1], "end_date")
    return start, end


def _print_execution_link(execution_id: str) -> None:
    if execution_id:
        console.print(f"Link: {execution_link(execution_id)}", soft_wrap=True)


def _print_executions(executions: list[execution_pb.Execution], include_backtest: bool = False) -> None:
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
    without_data: list[str] = []
    for entity in broker.backtest.list_backtests():
        if not entity.periods:
            without_data.append(entity.name)
            continue
        start, end = _backtest_range(entity)
        table.add_row(entity.name, start, end, entity.benchmark, _shown_provider(entity))
    console.print(table)
    # Listed after the table, so the backtests that can run are still shown.
    # A backtest without data is not a failure of `list`, so it exits 0.
    for name in without_data:
        console.print(
            f"⚠ {name} has no data. Run `fbull backtest delete {name}`, then `fbull backtest create`.",
            markup=False,
            highlight=False,
            soft_wrap=True,
        )


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
        backtest_service_pb.CreateBacktestRequest.CreateFromETF
        | backtest_service_pb.CreateBacktestRequest.CreateFromBacktest
    )
    if file:
        if not file.exists():
            raise FileNotFoundError(f"File {file} not found")
        with open(file, "r") as f:
            data = yaml.safe_load(f)
        chosen, source = _choose_provider(flag_provider, data.get("provider"), file.name)
        if chosen is None:
            raise typer.Exit(code=1)
        entity = backtest_service_pb.CreateBacktestRequest.CreateFromBacktest(
            backtest=backtest_pb.Backtest(
                name=data["name"],
                benchmark=data["benchmark"],
                provider=chosen,
                profile=_optional_profile(data.get("profile")),
                periods=[
                    backtest_pb.Backtest.Period(
                        start_date=from_pydate_to_proto_date(date.fromisoformat(period["start_date"])),
                        end_date=from_pydate_to_proto_date(date.fromisoformat(period["end_date"])),
                        assets=[
                            backtest_pb.Backtest.Period.Asset(
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
        if chosen is None:
            raise typer.Exit(code=1)
        entity = backtest_service_pb.CreateBacktestRequest.CreateFromETF(
            name=name,
            base_etf=base_etf,
            start_date=from_pydate_to_proto_date(start),
            end_date=from_pydate_to_proto_date(end),
            provider=chosen,
        )

    console.print(f"Using provider: {chosen} ({source})", markup=False)

    created: backtest_pb.Backtest | None = None
    stored = False
    failure: str | None = None
    with LiveProgress() as progress:
        try:
            for backtest_rsp in broker.backtest.create(entity=entity):
                progress.update_from_message(backtest_rsp)
                creating = backtest_rsp.CREATING
                if creating is not None and creating.state == common_pb.State.COMPLETED:
                    stored = True
                storing = backtest_rsp.STORING
                if storing is not None and storing.state == common_pb.State.COMPLETED:
                    created = backtest_rsp.backtest
        except grpc.RpcError as exc:
            failure = _rpc_details(exc)
            name = _entity_name(entity)
            if stored and created is None and name != "":
                failure = _discard_partial_backtest(name, failure)
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
    entity = get_backtest_or_fail(name)
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
    # Check every name before asking or deleting anything, so a typo stops the whole command.
    for name in names:
        get_backtest_or_fail(name, suggest_create=False)
    for name in names:
        if not force:
            shall_delete = Confirm.ask(f"Are you sure you want to delete backtest {name}?", default=False)
            if not shall_delete:
                return

        broker.backtest.delete(name)


def _parse_run_dates(
    entity: backtest_pb.Backtest,
    start: str | None,
    end: str | None,
) -> tuple[common_pb.Date, common_pb.Date] | None:
    if not entity.periods:
        # Same wording as the broker's refusal, so both paths read the same.
        _fail(
            f"Backtest '{entity.name}' has no data to run. Delete it with `fbull backtest delete {entity.name}`, "
            "then create it again with `fbull backtest create`."
        )

    start_date = entity.periods[0].start_date
    end_date = entity.periods[-1].end_date
    if start_date is None or end_date is None:
        console.print(f"[red]Backtest {entity.name} is missing a period date")
        return None
    if start:
        try:
            start_date = from_pydate_to_proto_date(date.fromisoformat(start))
        except ValueError:
            console.print(f"[red]Invalid start date: [yellow]{start}")
            return None

    if end:
        try:
            end_date = from_pydate_to_proto_date(date.fromisoformat(end))
        except ValueError:
            console.print(f"[red]Invalid end date: [yellow]{end}")
            return None

    _check_run_dates_in_range(entity, start_date, end_date, start=bool(start), end=bool(end))
    return start_date, end_date


def _check_run_dates_in_range(
    entity: backtest_pb.Backtest,
    start_date: common_pb.Date,
    end_date: common_pb.Date,
    *,
    start: bool,
    end: bool,
) -> None:
    """Fail before any session starts when --start/--end fall outside the backtest, one ✖ line per bad flag."""
    bt_start = pb_utils.from_proto_date_to_pydate(entity.periods[0].start_date)
    bt_end = pb_utils.from_proto_date_to_pydate(entity.periods[-1].end_date)
    run_start = pb_utils.from_proto_date_to_pydate(start_date)
    run_end = pb_utils.from_proto_date_to_pydate(end_date)
    hint = (
        f"Pick dates between {bt_start.isoformat()} and {bt_end.isoformat()}, "
        "or leave out --start and --end to run the whole period."
    )

    problems: list[str] = []
    for flag, given, used in (("--start", start, run_start), ("--end", end, run_end)):
        if not given:
            continue
        if used < bt_start:
            problems.append(f"{flag} {used.isoformat()} is before backtest '{entity.name}' begins. {hint}")
        elif used > bt_end:
            problems.append(f"{flag} {used.isoformat()} is after backtest '{entity.name}' ends. {hint}")
    if not problems and run_start > run_end:
        problems.append(
            f"--start {run_start.isoformat()} is after --end {run_end.isoformat()}. "
            "Pick a start date on or before the end date."
        )
    if not problems:
        return
    for problem in problems:
        console.print(f"✖ {problem}", markup=False, highlight=False, soft_wrap=True)
    raise typer.Exit(code=1)


# Codes the broker uses to refuse a run it will not start. Any other code is a fault.
_BROKER_RUN_REJECTIONS = (grpc.StatusCode.INVALID_ARGUMENT, grpc.StatusCode.FAILED_PRECONDITION)


def get_backtest_or_fail(name: str, *, suggest_create: bool = True) -> backtest_pb.Backtest:
    """Look the backtest up, failing with one plain line when the name doesn't exist.

    The line points at `fbull backtest list` and, when suggest_create is set, at
    `fbull backtest create`. Callers that remove a backtest leave the create hint out.

    Any other lookup error is raised unchanged, so it is never reported as a missing backtest.
    """
    try:
        return broker.backtest.get(name)
    except grpc.RpcError as exc:
        code = getattr(exc, "code", None)
        if callable(code) and code() == grpc.StatusCode.NOT_FOUND:
            if suggest_create:
                _fail(
                    f"There's no backtest named '{name}'. Run `fbull backtest list` to see your "
                    "backtests, or create it with `fbull backtest create`."
                )
            _fail(f"There's no backtest named '{name}'. Run `fbull backtest list` to see your backtests.")
        raise


@backtest.command()
def run(
    name: Annotated[str, typer.Argument(help="name of the backtest")],
    file_path: Annotated[str, typer.Argument(help="path to the strategy file")],
    start: Annotated[str | None, typer.Option(help="start date of the backtest")] = None,
    end: Annotated[str | None, typer.Option(help="end date of the backtest")] = None,
    dry: Annotated[bool, typer.Option("--dry", help="print the execution plan without running")] = False,
):
    entity = get_backtest_or_fail(name)
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
    rejected: str | None = None
    with LiveProgress() as progress:
        has_run = False
        for msg in broker.backtest.create_session(name):
            progress.update_from_message(msg)
            backtest_session = msg.session
            if backtest_session is not None and backtest_session.port and not has_run:
                has_run = True
                try:
                    with Foreverbull(models.Algorithm.from_file_path(file_path)).backtest_session(
                        entity, backtest_session
                    ) as session:
                        with progress.bar_progress("execution") as bar_progress:
                            for execution_id, message in session.run_execution(
                                start_date,
                                end_date,
                            ):
                                bar_progress(message.num_current_period, message.num_total_periods)
                        log.info(f"Execution completed for {session.backtest.name}")
                except grpc.RpcError as exc:
                    # The broker checks the dates and that the backtest has data;
                    # its message is the whole error.
                    code = getattr(exc, "code", None)
                    if not callable(code) or code() not in _BROKER_RUN_REJECTIONS:
                        raise
                    rejected = _rpc_details(exc)
                    progress.transient = True

    if rejected is not None:
        _fail(rejected)
    if execution_id is None:
        _fail(progress.failure or "The backtest session ended before an execution started.")
    _, periods = broker.backtest.get_execution(execution_id)
    print_run_summary(
        backtest_name=entity.name,
        benchmark=entity.benchmark,
        provider=_shown_provider(entity),
        start_iso=start_iso,
        end_iso=end_iso,
        execution_id=execution_id,
        periods=list(periods),
    )


def _resolve_execution_id(raw: str) -> str:
    """Accept the full UUID or a prefix of it, such as the short id `backtest run` prints."""
    prefix = raw.strip()
    if not prefix:
        _fail("Give an execution ID. List them with `fbull backtest executions`.")
    if len(prefix) >= _FULL_EXECUTION_ID_LENGTH:
        return prefix
    matches = [current.id for current in broker.backtest.list_executions() if current.id.startswith(prefix)]
    if not matches:
        _fail(f"No execution ID starts with {prefix!r}. List them with `fbull backtest executions`.")
    if len(matches) > 1:
        shown = ", ".join(distinguishing_execution_ids(matches)[:_AMBIGUOUS_MATCHES_SHOWN])
        _fail(f"{prefix!r} matches {len(matches)} executions ({shown}). Use more characters of the ID.")
    return matches[0]


@backtest.command()
def executions(
    backtest: Annotated[str | None, typer.Option(help="name of the backtest")] = None,
    session: Annotated[str | None, typer.Option(help="id of the session")] = None,
):
    rows = broker.backtest.list_executions(backtest, session)
    _print_executions(rows, include_backtest=True)


@backtest.command()
def execution(
    id: Annotated[str, typer.Argument(help="id of the execution, or the short id `backtest run` prints")],
):
    entity, periods = broker.backtest.get_execution(_resolve_execution_id(id))

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
