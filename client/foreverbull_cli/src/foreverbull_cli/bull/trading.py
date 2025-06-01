import logging

import typer

from rich.live import Live
from rich.progress import Progress
from typing_extensions import Annotated

from foreverbull import models
from foreverbull.foreverbull import Foreverbull


trading = typer.Typer()
log = logging.getLogger().getChild(__name__)


@trading.command()
def run(
    file_path: Annotated[str, typer.Argument(help="name of the trading strategy")],
    dry: Annotated[bool, typer.Option(help="dry run the strategy")] = False,
    yes: Annotated[bool, typer.Option(help="skip confirmation prompt")] = False,
):
    live = Live(Progress())
    with Foreverbull(models.Algorithm.from_file_path(file_path)).trading_session([]) as session, live:
        assert type(session) is not None
        pass
