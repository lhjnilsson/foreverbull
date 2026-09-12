import logging
import os

from typing import Annotated

import typer

from rich.logging import RichHandler

from foreverbull_cli.bull.backtest import backtest
from foreverbull_cli.bull.trading import trading
from foreverbull_cli.output import console


cli = typer.Typer()

cli.add_typer(backtest, name="backtest")
cli.add_typer(trading, name="trading")

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
):
    level = _logging_level(verbose)
    os.environ["LOGGING_LEVEL"] = level
    logging.basicConfig(
        level=level,
        format="%(message)s",
        datefmt="[%X]",
        handlers=[RichHandler(markup=True, console=console)],
    )
