import os

import pytest
import typer

from click.testing import Result
from typer.testing import CliRunner


@pytest.fixture
def runner(fb_env):
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str], input=None, kwargs: dict | None = None) -> Result:
        if kwargs is None:
            kwargs = {}
        result = runner.invoke(app, args, input=input, **kwargs)
        if result.exception:
            raise result.exception
        if os.environ.get("ECHO"):
            print()
            print(result.stdout)

        return result

    return invoke
