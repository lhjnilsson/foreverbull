import logging

import typer

from rich.live import Live
from rich.progress import Progress
from typing_extensions import Annotated

from foreverbull import broker
from foreverbull import models
from foreverbull.foreverbull import Foreverbull


trading = typer.Typer()
log = logging.getLogger().getChild(__name__)


@trading.command()
def run(
    backtest_name: Annotated[str, typer.Argument(help="name of the backtest which symbols to trade")],
    file_path: Annotated[str, typer.Argument(help="path to the trading strategy file")],
    dry: Annotated[bool, typer.Option(help="dry run the strategy")] = False,
    yes: Annotated[bool, typer.Option(help="skip confirmation prompt")] = False,
):
    _backtest = broker.backtest.get(backtest_name)
    symbols = [s for s in _backtest.symbols]

    live = Live(Progress())
    with Foreverbull(models.Algorithm.from_file_path(file_path)).trading_session(symbols) as session, live:
        orders = session.run_trading(symbols)
        for order in orders:
            print(order)
