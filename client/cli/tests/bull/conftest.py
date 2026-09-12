import os

import pytest
import typer

from click.testing import Result
from typer.testing import CliRunner


@pytest.fixture
def runner(fb_env):
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str], input=None, kwargs: dict = {}) -> Result:
        result = runner.invoke(app, args, input=input, **kwargs)
        if result.exception:
            raise result.exception
        echo_stdout = os.environ.get("ECHO", False)
        if echo_stdout:
            print("")
            print(result.stdout)

        return result

    return invoke
