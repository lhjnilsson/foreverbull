import sys

from typing import Annotated

import typer

from foreverbull.broker.credentials import TokenFileError
from foreverbull.broker.credentials import save_token_file
from foreverbull_cli.output import console


token_app = typer.Typer(no_args_is_help=True)


@token_app.command("save")
def save(
    force: Annotated[bool, typer.Option("--force", help="Replace an existing token file")] = False,
):
    """Read a token from stdin and write ~/.foreverbull/token mode 0600."""
    try:
        path = save_token_file(sys.stdin.read(), force=force)
    except TokenFileError as exc:
        console.print(str(exc), style="red", highlight=False)
        raise typer.Exit(1) from None
    console.print(str(path), highlight=False)
