from unittest.mock import MagicMock

import pytest
import typer

from typer.testing import CliRunner

from foreverbull_cli.env import cli
from foreverbull_cli.env.environment import ContainerManager
from foreverbull_cli.env.environment import ForeverbullContainer
from foreverbull_cli.env.environment import GrafanaContainer
from foreverbull_cli.env.environment import MinioContainer
from foreverbull_cli.env.environment import NatsContainer
from foreverbull_cli.env.environment import PostgresContainer


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


def test_status(mocked_container_manager: MagicMock):
    runner = CliRunner()

    result = runner.invoke(cli.cli, "status")
    assert result.exit_code == 0


def test_create(mocked_container_manager: MagicMock):
    runner = CliRunner()

    result = runner.invoke(cli.cli, "create")
    assert result.exit_code == 0


def test_start(mocked_container_manager: MagicMock):
    runner = CliRunner()

    result = runner.invoke(cli.cli, "start")
    assert result.exit_code == 0


def test_stop(mocked_container_manager: MagicMock):
    runner = CliRunner()

    result = runner.invoke(cli.cli, "stop")
    assert result.exit_code == 0


def test_update(mocked_container_manager: MagicMock):
    runner = CliRunner()

    result = runner.invoke(cli.cli, "update")
    assert result.exit_code == 0
