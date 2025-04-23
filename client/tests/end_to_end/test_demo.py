import os

import pytest
import typer

from click.testing import Result
from typer.testing import CliRunner

from foreverbull_cli.bull.backtest import backtest as backtest_cli
from foreverbull_cli.env import cli as env_cli


@pytest.fixture
def runner():
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str], env: dict[str, str] = {}) -> Result:
        result = runner.invoke(app, args, env=env)
        echo_stdout = os.environ.get("ECHO", False)
        if echo_stdout:
            print("")
            print(result.stdout)

        return result

    return invoke


def test_start_stop(runner):
    result = runner(env_cli.cli, "stop --remove --purge")
    assert result.exit_code == 0

    result = runner(env_cli.cli, "create", env={"VERSION": "local"})
    assert result.exit_code == 0

    # Create Backtest
    result = runner(backtest_cli, "create example_algorithms/nasdaq.json")
    assert result.exit_code == 0

    # Run Backtest

    # List Backtest Results
