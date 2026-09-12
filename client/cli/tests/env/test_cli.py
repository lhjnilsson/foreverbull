import os

from unittest.mock import MagicMock

import pytest
import typer

from click.testing import Result
from foreverbull_cli.env import cli
from foreverbull_cli.env.environment import ContainerManager
from foreverbull_cli.env.environment import WorkloadRow
from typer.testing import CliRunner


pytestmark = pytest.mark.usefixtures("fb_env")


STATUS_ROWS = [
    WorkloadRow("Postgres", "postgres", "running", "postgres:13.3-alpine", "abc"),
    WorkloadRow("NATS", "nats", "running", "nats:2.10-alpine", "def"),
    WorkloadRow("Minio", "minio", "running", "minio/minio:latest", "ghi"),
    WorkloadRow("Zipline", "zipline", "running", "lhjnilsson/zipline:latest", "jkl"),
    WorkloadRow("Foreverbull", "foreverbull", "running", "lhjnilsson/foreverbull:latest", "mno"),
    WorkloadRow("Grafana", "grafana", "running", "lhjnilsson/fb-grafana:latest", "pqr"),
]


@pytest.fixture
def runner(fb_env):
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str]) -> Result:
        result = runner.invoke(app, args)
        echo_stdout = os.environ.get("ECHO", False)
        if echo_stdout:
            print("")
            print(result.stdout)

        return result

    return invoke


@pytest.fixture
def mocked_container_manager():
    container_manager = MagicMock(spec=ContainerManager)
    container_manager.status_rows.return_value = STATUS_ROWS

    def initialize(ctx: typer.Context):
        ctx.obj = container_manager

    assert cli.cli.registered_callback
    cli.cli.registered_callback.callback = initialize
    return container_manager


def test_status(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "status")
    assert result.exit_code == 0
    mocked_container_manager.status_rows.assert_called()
    assert "Postgres" in result.stdout
    assert "Zipline" in result.stdout
    assert "Grafana" in result.stdout


def test_create(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "create")
    assert result.exit_code == 0
    mocked_container_manager.verify_images.assert_called_once()
    mocked_container_manager.create.assert_called_once_with(start=True)
    mocked_container_manager.start.assert_called_once()


def test_create_no_start(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "create --no-start")
    assert result.exit_code == 0
    mocked_container_manager.create.assert_called_once_with(start=False)
    mocked_container_manager.start.assert_not_called()


def test_start(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "start")
    assert result.exit_code == 0
    mocked_container_manager.start.assert_called_once()


def test_stop(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "stop")
    assert result.exit_code == 0
    mocked_container_manager.stop.assert_called_once()
    mocked_container_manager.remove.assert_not_called()


def test_stop_remove(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "stop --remove")
    assert result.exit_code == 0
    mocked_container_manager.stop.assert_called_once()
    mocked_container_manager.remove.assert_called_once_with(False)


def test_stop_remove_purge(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "stop --remove --purge")
    assert result.exit_code == 0
    mocked_container_manager.remove.assert_called_once_with(True)


def test_update(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "update")
    assert result.exit_code == 0
    mocked_container_manager.update.assert_called_once()
