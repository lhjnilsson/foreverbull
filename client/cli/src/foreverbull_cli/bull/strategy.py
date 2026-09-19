import json

from enum import Enum
from typing import Annotated

import typer

from rich.console import Console
from rich.table import Table

from foreverbull import broker
from foreverbull.pb.foreverbull.strategy.v1 import strategy_pb2
from foreverbull_cli.output import LiveProgress
from foreverbull_cli.output import console
from foreverbull_cli.output import execution_link
from foreverbull_cli.output import format_date_field
from foreverbull_cli.output import format_optional_float


strategy = typer.Typer()
version_cli = typer.Typer()
strategy.add_typer(version_cli, name="version")


class DeployMode(str, Enum):
    PAPER = "paper"
    LIVE = "live"


def _optional_text(message, field: str) -> str:
    if not message.HasField(field):
        return ""
    return str(getattr(message, field))


def _enum_name(enum_cls, value) -> str:
    try:
        return enum_cls.Name(value)
    except ValueError:
        return str(value)


def _format_timestamp(message, field: str) -> str:
    if not message.HasField(field):
        return ""
    ts = getattr(message, field)
    try:
        return ts.ToDatetime().isoformat()
    except (OverflowError, OSError, ValueError):
        return ""


def _parse_symbols(symbols: str | None) -> list[str]:
    if not symbols:
        return []
    return [item.strip() for item in symbols.split(",") if item.strip()]


def _parse_parameters(raw: str | None) -> dict | None:
    if not raw:
        return None
    try:
        loaded = json.loads(raw)
    except json.JSONDecodeError:
        console.print(f"[red]Invalid parameters JSON: [yellow]{raw}")
        return None
    if not isinstance(loaded, dict):
        console.print("[red]parameters must be a JSON object")
        return None
    return loaded


def _print_strategy(entity: strategy_pb2.Strategy, title: str = "Strategy") -> None:
    table = Table(title=title)
    table.add_column("ID")
    table.add_column("Name")
    table.add_column("Description")
    table.add_column("Created")
    table.add_row(
        entity.id,
        entity.name,
        _optional_text(entity, "description"),
        _format_timestamp(entity, "created_at"),
    )
    console.print(table)


def _print_version(entity: strategy_pb2.Version) -> None:
    table = Table(title="Version")
    table.add_column("ID")
    table.add_column("Strategy")
    table.add_column("Rev")
    table.add_column("Image")
    table.add_column("Created")
    table.add_row(
        entity.id,
        entity.strategy_name,
        str(entity.rev),
        entity.service_image,
        _format_timestamp(entity, "created_at"),
    )
    console.print(table)


def _print_experiment_links(experiments: list[strategy_pb2.Experiment]) -> None:
    for experiment in experiments:
        execution_id = _optional_text(experiment, "execution_id")
        if execution_id:
            console.print(f"Link: {execution_link(execution_id)}")


def _format_version(experiment: strategy_pb2.Experiment) -> str:
    return f"{experiment.version_rev} ({experiment.version_id})"


def _format_stat(experiment: strategy_pb2.Experiment, field: str, *, optional: bool = False) -> str:
    if not experiment.HasField("stats"):
        return ""
    if optional:
        return format_optional_float(experiment.stats, field)
    return f"{getattr(experiment.stats, field):.5f}"


_EXPERIMENT_TABLE_WIDTH = 160


def _print_compare_table(table: Table) -> None:
    # Dumb/piped terminals report width 80 and would crop Sharpe/drawdown.
    printer = Console(
        file=console.file,
        width=max(console.width, _EXPERIMENT_TABLE_WIDTH),
        height=max(console.height, 25),
        highlight=False,
    )
    printer.print(table)


def _print_experiments(experiments: list[strategy_pb2.Experiment], title: str = "Experiments") -> None:
    table = Table(title=title)
    table.add_column("ID")
    table.add_column("Strategy")
    table.add_column("Version")
    table.add_column("Backtest")
    table.add_column("Status")
    table.add_column("Execution")
    table.add_column("Sharpe")
    table.add_column("Max Drawdown")
    table.add_column("End Value")
    table.add_column("Benchmark")
    for experiment in experiments:
        table.add_row(
            experiment.id,
            experiment.strategy_name,
            _format_version(experiment),
            experiment.backtest_name,
            _enum_name(strategy_pb2.Experiment.Status, experiment.status),
            _optional_text(experiment, "execution_id"),
            _format_stat(experiment, "sharpe", optional=True),
            _format_stat(experiment, "max_drawdown"),
            _format_stat(experiment, "portfolio_value"),
            _format_stat(experiment, "benchmark_period_return", optional=True),
        )
    _print_compare_table(table)
    _print_experiment_links(experiments)


def _print_deployment(entity: strategy_pb2.Deployment, title: str = "Deployment") -> None:
    table = Table(title=title)
    table.add_column("ID")
    table.add_column("Strategy")
    table.add_column("Rev")
    table.add_column("Mode")
    table.add_column("Dry Run")
    table.add_column("Status")
    table.add_column("Calendar")
    table.add_column("Symbols", overflow="fold")
    table.add_column("Last Session")
    table.add_row(
        entity.id,
        entity.strategy_name,
        str(entity.version_rev),
        _enum_name(strategy_pb2.Deployment.Mode, entity.mode),
        str(entity.dry_run).lower(),
        _enum_name(strategy_pb2.Deployment.Status, entity.status),
        entity.calendar_code,
        ",".join(entity.symbols),
        format_date_field(entity, "last_session_date"),
    )
    console.print(table)


def _print_deployment_runs(runs: list[strategy_pb2.DeploymentRun]) -> None:
    table = Table(title="Deployment Runs")
    table.add_column("ID")
    table.add_column("Deployment")
    table.add_column("Session")
    table.add_column("Orders")
    table.add_column("Skipped")
    table.add_column("Error")
    for run in runs:
        table.add_row(
            run.id,
            run.deployment_id,
            format_date_field(run, "session_date"),
            str(len(run.orders)),
            "yes" if run.skipped else "no",
            _optional_text(run, "error"),
        )
    console.print(table)


@strategy.command()
def create(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
    description: Annotated[str | None, typer.Option(help="optional description")] = None,
):
    entity = broker.strategy.create_strategy(name, description=description)
    _print_strategy(entity, title="Created Strategy")


@version_cli.command("add")
def add_version(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
    image: Annotated[str, typer.Option("--image", help="READY service image")],
    parameters: Annotated[str | None, typer.Option(help="JSON object of function parameter overrides")] = None,
):
    parsed = _parse_parameters(parameters)
    if parameters and parsed is None:
        return
    entity = broker.strategy.create_version(name, image, parameters=parsed)
    _print_version(entity)


@strategy.command()
def experiment(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
    backtest: Annotated[
        str,
        typer.Option("--backtest", help="backtest universe (spy, eu-small, commodities, ...)"),
    ],
    rev: Annotated[int | None, typer.Option("--rev", help="strategy version revision")] = None,
    version: Annotated[str | None, typer.Option("--version", help="strategy version id")] = None,
):
    created: strategy_pb2.Experiment | None = None
    with LiveProgress() as progress:
        for rsp in broker.strategy.create_experiment(
            strategy_name=name,
            backtest_name=backtest,
            version_id=version,
            version_rev=rev,
        ):
            progress.update_from_message(rsp)
            if rsp.HasField("experiment"):
                created = rsp.experiment

    if created is None:
        return
    _print_experiments([created], title="Experiment")


@strategy.command()
def experiments(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
    backtest: Annotated[str | None, typer.Option("--backtest", help="filter by backtest universe")] = None,
    version: Annotated[str | None, typer.Option("--version", help="filter by version id")] = None,
):
    rows = broker.strategy.list_experiments(
        strategy_name=name,
        version_id=version,
        backtest_name=backtest,
    )
    _print_experiments(rows)


@strategy.command()
def deploy(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
    dry_run: Annotated[
        bool,
        typer.Option("--dry-run/--no-dry-run", help="run Process but skip PlaceOrder (required for PoC)"),
    ] = True,
    mode: Annotated[DeployMode, typer.Option(help="paper (default) or live")] = DeployMode.PAPER,
    rev: Annotated[int | None, typer.Option("--rev", help="strategy version revision")] = None,
    version: Annotated[str | None, typer.Option("--version", help="strategy version id")] = None,
    backend: Annotated[str | None, typer.Option(help="trading backend name")] = None,
    calendar: Annotated[str | None, typer.Option(help="trading calendar code (e.g. XNYS)")] = None,
    symbols: Annotated[str | None, typer.Option(help="comma-separated symbols the deployment may trade")] = None,
):
    if mode == DeployMode.LIVE:
        console.print("[red]live mode is not enabled in this PoC; deploy with --mode paper --dry-run")
        return
    if not dry_run:
        console.print("[red]PoC deployments must use --dry-run; PlaceOrder is disabled")
        return

    entity = broker.strategy.deploy(
        strategy_name=name,
        version_id=version,
        version_rev=rev,
        dry_run=True,
        backend=backend,
        calendar_code=calendar,
        symbols=_parse_symbols(symbols),
    )
    _print_deployment(entity)


@strategy.command()
def stop(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
    deployment: Annotated[str | None, typer.Option(help="deployment id (defaults to the running one)")] = None,
):
    entity = broker.strategy.stop_deployment(name, deployment_id=deployment)
    _print_deployment(entity, title="Stopped Deployment")


@strategy.command()
def status(
    name: Annotated[str, typer.Argument(help="name of the strategy")],
):
    entity = broker.strategy.get_strategy(name)
    _print_strategy(entity)
    _print_experiments(broker.strategy.list_experiments(strategy_name=name))
    _print_deployment_runs(broker.strategy.list_deployment_runs(strategy_name=name))
