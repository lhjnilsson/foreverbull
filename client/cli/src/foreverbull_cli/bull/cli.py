import logging
import os

from typing import Annotated

import typer

from rich.logging import RichHandler

from foreverbull.broker.credentials import set_cli_token
from foreverbull_cli.bull.backtest import backtest
from foreverbull_cli.bull.strategy import strategy
from foreverbull_cli.bull.token import token_app
from foreverbull_cli.bull.trading import trading
from foreverbull_cli.output import console


cli = typer.Typer()

cli.add_typer(backtest, name="backtest")
cli.add_typer(strategy, name="strategy")
cli.add_typer(trading, name="trading")
cli.add_typer(token_app, name="token")

_LOG_LEVELS = {
    0: "ERROR",
    1: "WARNING",
    2: "INFO",
    3: "DEBUG",
}


def _logging_level(verbose: int) -> str:
    return _LOG_LEVELS.get(verbose, "DEBUG")


@cli.callback()
def setup_logging(
    verbose: Annotated[
        int,
        typer.Option("--verbose", "-v", count=True, help="Increase logging verbosity; repeat for more detail"),
    ] = 0,
    token: Annotated[
        str | None,
        typer.Option("--token", help="Bearer for this command. Not stored."),
    ] = None,
):
    set_cli_token(token)
    level = _logging_level(verbose)
    os.environ["LOGGING_LEVEL"] = level
    logging.basicConfig(
        level=level,
        format="%(message)s",
        datefmt="[%X]",
        handlers=[RichHandler(markup=True, console=console)],
    )
