import os

from unittest.mock import MagicMock

import pytest
import typer

from click.testing import Result
from foreverbull_cli.env import cli
from foreverbull_cli.env.environment import ContainerManager
from foreverbull_cli.env.environment import ForeverbullContainer
from foreverbull_cli.env.environment import GrafanaContainer
from foreverbull_cli.env.environment import MinioContainer
from foreverbull_cli.env.environment import NatsContainer
from foreverbull_cli.env.environment import PostgresContainer
from typer.testing import CliRunner


pytestmark = pytest.mark.usefixtures("fb_env")


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

    postgres = MagicMock(spec=PostgresContainer)
    postgres.name = "testing"
    postgres.status.return_value = "OK"
    postgres.image_version.return_value = "OK"
    postgres.container_id.return_value = "OK"

    nats = MagicMock(spec=NatsContainer)
    nats.name = "testing"
    nats.status.return_value = "OK"
    nats.image_version.return_value = "OK"
    nats.container_id.return_value = "OK"

    minio = MagicMock(spec=MinioContainer)
    minio.name = "testing"
    minio.status.return_value = "OK"
    minio.image_version.return_value = "OK"
    minio.container_id.return_value = "OK"

    grafana = MagicMock(spec=GrafanaContainer)
    grafana.name = "testing"
    grafana.status.return_value = "OK"
    grafana.image_version.return_value = "OK"
    grafana.container_id.return_value = "OK"

    foreverbull = MagicMock(spec=ForeverbullContainer)
    foreverbull.name = "testing"
    foreverbull.status.return_value = "OK"
    foreverbull.image_version.return_value = "OK"
    foreverbull.container_id.return_value = "OK"

    container_manager.postgres = postgres
    container_manager.nats = nats
    container_manager.minio = minio
    container_manager.grafana = grafana
    container_manager.foreverbull = foreverbull

    def initialize(ctx: typer.Context):
        ctx.obj = container_manager

    # Replace with mocked Container Manager
    assert cli.cli.registered_callback
    cli.cli.registered_callback.callback = initialize


def test_status(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "status")
    assert result.exit_code == 0


def test_create(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "create")
    assert result.exit_code == 0


def test_start(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "start")
    assert result.exit_code == 0


def test_stop(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "stop")
    assert result.exit_code == 0


def test_update(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "update")
    assert result.exit_code == 0
