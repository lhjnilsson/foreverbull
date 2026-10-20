import os
import pathlib
import re

import pytest
import typer

from click.testing import Result
from cookiecutter.main import cookiecutter
from foreverbull_cli.bull.backtest import backtest as backtest_cli
from foreverbull_cli.env import cli as env_cli
from typer.testing import CliRunner


REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
TEMPLATES = REPO_ROOT / "client" / "templates" / "simple"


@pytest.fixture
def runner():
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str], env: dict[str, str] | None = None) -> Result:
        if env is None:
            env = {}
        result = runner.invoke(app, args, env=env)
        if os.environ.get("ECHO"):
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
    return project / "crossover.py", project / "backtest.yaml"


def test_start_stop(runner, simple_algorithm):
    """Install Kind with fbenv and run the simple template through fbull.

    CI retags the images built on this commit to `:local` and runs this test.
    """
    crossover, backtest_file = simple_algorithm

    result = runner(env_cli.cli, "stop --remove --purge")
    assert result.exit_code == 0, result.output

    result = runner(env_cli.cli, "create", env={"VERSION": "local"})
    assert result.exit_code == 0, result.output

    result = runner(env_cli.cli, "start", env={"VERSION": "local"})
    assert result.exit_code == 0, result.output

    result = runner(backtest_cli, f"create --file {backtest_file}")
    assert result.exit_code == 0, result.output
    assert "Using provider: yahoo (default)" in result.output

    result = runner(backtest_cli, f"run demo {crossover}")
    assert result.exit_code == 0, result.output
    assert "Backtest finished: demo · 2021-01-01 → 2021-03-31 · provider: yahoo" in result.output
    assert "Return" in result.output

    match = re.search(r"fbull backtest execution ([0-9a-f]+)", result.output)
    assert match is not None, result.output
    execution = runner(backtest_cli, f"execution {match.group(1)}")
    assert execution.exit_code == 0, execution.output
    assert "Portfolio" in execution.output
    assert "2021-" in execution.output
