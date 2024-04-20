import typer
from rich.console import Console

from .backtest import backtest
from .env import env
from .service import service

cli = typer.Typer()

cli.add_typer(
    backtest,
    name="backtest",
    help="asfk",
)
cli.add_typer(
    service,
    name="service",
)
cli.add_typer(
    env,
    name="env",
)

std = Console()
std_err = Console(stderr=True)
