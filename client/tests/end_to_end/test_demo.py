import os
import pathlib

import pytest
import typer

from click.testing import Result
from cookiecutter.main import cookiecutter
from foreverbull_cli.bull.backtest import backtest as backtest_cli
from foreverbull_cli.env import cli as env_cli
from typer.testing import CliRunner


TEMPLATES = pathlib.Path(__file__).resolve().parents[2] / "templates" / "simple"


@pytest.fixture
def runner():
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str], env: dict[str, str] = {}) -> Result:
        result = runner.invoke(app, args, env=env)
        echo_stdout = os.environ.get("ECHO", False)
        if echo_stdout:
            print()
            print(result.stdout)

        return result

    return invoke


@pytest.fixture
def simple_algorithm(tmp_path):
    cookiecutter(
        str(TEMPLATES),
        no_input=True,
        extra_context={
            "project_name": "simple",
            "project_slug": "simple",
            "install_dependencies": False,
            "run_setup": False,
        },
        output_dir=str(tmp_path),
        overwrite_if_exists=True,
    )
    project = tmp_path / "simple"
    return project / "simple" / "crossover.py", project / "backtest.yaml"


def test_start_stop(runner, simple_algorithm):
    crossover, backtest_file = simple_algorithm

    result = runner(env_cli.cli, "stop --remove --purge")
    assert result.exit_code == 0

    result = runner(env_cli.cli, "create", env={"VERSION": "local"})
    assert result.exit_code == 0

    result = runner(env_cli.cli, "start", env={"VERSION": "local"})
    assert result.exit_code == 0

    result = runner(backtest_cli, f"create --file {backtest_file}")
    assert result.exit_code == 0

    result = runner(backtest_cli, f"run demo {crossover} --yes")
    assert result.exit_code == 0
