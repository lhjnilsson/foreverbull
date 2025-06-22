import os

import pytest
import typer

from click.testing import Result
from typer.testing import CliRunner


@pytest.fixture
def runner():
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str], kwargs: dict = {}) -> Result:
        result = runner.invoke(app, args, **kwargs)
        if result.exception:
            raise result.exception
        echo_stdout = os.environ.get("ECHO", False)
        if echo_stdout:
            print("")
            print(result.stdout)

        return result

    return invoke
