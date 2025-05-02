import json
import tempfile

from pathlib import Path

import typer

from click import Command
from rich.console import Console
from rich.prompt import Confirm
from rich.prompt import Prompt

from foreverbull_cli.bull.backtest import create as create_backtest
from foreverbull_cli.env.cli import create
from foreverbull_cli.env.cli import initialize
from foreverbull_cli.env.cli import stop
from foreverbull_cli.env.environment import EnvironmentExists


console = Console()


def setup():
    setup_environment = Confirm.ask("Do you want to setup the environment?", default=True)
    if setup_environment:
        ctx = typer.Context(Command(name=None))
        initialize(ctx)
        try:
            create(ctx, True)
        except EnvironmentExists:
            console.print("Environment already exists")
            remove = Confirm.ask("Do you want to remove the environment?", default=False)
            if not remove:
                raise SystemExit()
            stop(ctx, True, True)
            create(ctx, True)
        create_nasdaq_100_backtest = Confirm.ask(
            "Do you want to create a backtest with nasdaq 100 assets?", default=True
        )
    else:
        create_nasdaq_100_backtest = False

    if create_nasdaq_100_backtest:
        nasdaq_100_file = Path(__file__).resolve().parent / "nasdaq.json"
        with open(nasdaq_100_file, "r") as f:
            nasdaq_100_cfg = json.load(f)

        nasdaq_100_name = Prompt.ask("What is the name of the backtest?", default="nasdaq_100")
        nasdaq_100_cfg["start_date"] = Prompt.ask(
            "What is the start date of the nasdaq 100 backtest?", default=nasdaq_100_cfg["start_date"]
        )
        nasdaq_100_cfg["end_date"] = Prompt.ask(
            "What is the end date of the nasdaq 100 backtest?", default=nasdaq_100_cfg["end_date"]
        )
        with tempfile.NamedTemporaryFile("w") as fr:
            json.dump(nasdaq_100_cfg, fr)
            fr.flush()
            create_backtest(f.name, nasdaq_100_name)

    console.print("Setup completed!")
    console.print("run `[bold]fbenv[/bold]` to control the environment")
    console.print("run `[bold]fbull[/bold]` to run backtests")
    console.print("you can always use --help command, eg: [bold]fbull --help[/bold]")


if __name__ == "__main__":
    setup()
