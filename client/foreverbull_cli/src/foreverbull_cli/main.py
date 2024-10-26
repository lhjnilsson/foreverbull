import typer

from foreverbull_cli.backtest import backtest
from foreverbull_cli.output import console
from foreverbull_cli.env import env
import logging
from rich.logging import RichHandler

cli = typer.Typer()

cli.add_typer(backtest, name="backtest")
cli.add_typer(env, name="env")


@cli.callback()
def setup_logging(
    ctx: typer.Context,
    verbose: bool = typer.Option("False", "--verbose", "-v"),
    very_verbose: bool = typer.Option("False", "--very-verbose", "-vv"),
):
    if very_verbose:
        level = "NOSET"
    elif verbose:
        level = "DEBUG"
    else:
        level = "WARNING"
    logging.basicConfig(
        level=level,
        format="%(message)s",
        datefmt="[%X]",
        handlers=[RichHandler(markup=True, console=console)],
    )


def main():
    cli()
