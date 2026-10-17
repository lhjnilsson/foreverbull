import os

from unittest.mock import MagicMock
from unittest.mock import PropertyMock

import pytest
import typer

from click.testing import Result
from foreverbull_cli.env import cli
from foreverbull_cli.env.environment import BrokerDiagnosis
from foreverbull_cli.env.environment import ClusterManager
from foreverbull_cli.env.environment import CommandError
from foreverbull_cli.env.environment import ImagePullError
from foreverbull_cli.env.environment import MissingToolError
from foreverbull_cli.env.environment import VersionError
from foreverbull_cli.env.environment import WorkloadRow
from foreverbull_cli.env.preflight import CheckState
from foreverbull_cli.env.preflight import HostCheck
from foreverbull_cli.env.preflight import PreflightError
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

HOST_CHECKS = [
    HostCheck("kind", CheckState.PASS, "kind v0.24.0 go1.22.6 linux/amd64"),
    HostCheck("docker", CheckState.PASS, "27.3.1"),
    HostCheck("Docker daemon", CheckState.PASS, "running (server 27.3.1)"),
    HostCheck("Docker memory", CheckState.PASS, "7.7 GB available to Docker"),
    HostCheck(
        "Host ports",
        CheckState.FAIL,
        "Port 5432 (Postgres) is already in use.",
        "Stop the process that holds it.",
    ),
]
PORT_FAILURE = PreflightError(HOST_CHECKS[-1])


@pytest.fixture
def runner(fb_env):
    runner = CliRunner()

    def invoke(app: typer.Typer, args: list[str]) -> Result:
        result = runner.invoke(app, args)
        if os.environ.get("ECHO"):
            print()
            print(result.stdout)

        return result

    return invoke


@pytest.fixture
def mocked_container_manager():
    container_manager = MagicMock(spec=ClusterManager)
    container_manager.status_rows.return_value = STATUS_ROWS
    container_manager.host_checks.return_value = HOST_CHECKS
    container_manager.broker_diagnosis.return_value = None
    container_manager.config = MagicMock()
    container_manager.config.cluster = "local"
    container_manager.config.namespace = "foreverbull"
    container_manager.config.kube_context = "kind-foreverbull"
    container_manager.config.broker_address = "localhost:50055"

    def initialize(ctx: typer.Context):
        ctx.obj = container_manager

    assert cli.cli.registered_callback
    original = cli.cli.registered_callback.callback
    cli.cli.registered_callback.callback = initialize
    yield container_manager
    cli.cli.registered_callback.callback = original


def test_status(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "status")
    assert result.exit_code == 0
    mocked_container_manager.status_rows.assert_called()
    assert "Postgres" in result.stdout
    assert "Zipline" in result.stdout
    assert "Grafana" in result.stdout


def test_status_lists_host_checks_above_workloads(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "status")
    assert result.exit_code == 0
    mocked_container_manager.preflight.assert_not_called()
    assert "Host checks" in result.stdout
    assert "Docker memory" in result.stdout
    assert "Port 5432 (Postgres) is already in use." in result.stdout
    assert "Stop the process that holds it." in result.stdout
    assert result.stdout.index("Host checks") < result.stdout.index("Environment Status")


def test_status_without_kubectl_still_shows_host_checks(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.status_rows.side_effect = MissingToolError("kubectl")
    result = runner(cli.cli, "status")
    assert result.exit_code == 0
    assert "Host checks" in result.stdout
    assert "Workload status needs kubectl. kubectl is not installed." in result.stdout


def test_status_leads_broker_error_with_advice(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.broker_diagnosis.return_value = BrokerDiagnosis(
        "The broker can't reach its database. Run `uv run fbenv stop && uv run fbenv start`.",
        "could not reach postgres at postgres:5432 after 20 attempts",
    )
    result = runner(cli.cli, "status")
    assert result.exit_code == 0
    advice = result.stdout.index("The broker can't reach its database.")
    detail = result.stdout.index("could not reach postgres at postgres:5432")
    assert advice < detail


def test_create_stops_on_failed_host_check(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.preflight.side_effect = PORT_FAILURE
    result = runner(cli.cli, "create")
    assert result.exit_code == 1
    assert "✖ Port 5432 (Postgres) is already in use." in result.stdout
    assert "Nothing was changed." in result.stdout
    assert "Traceback" not in result.stdout
    mocked_container_manager.verify_images.assert_not_called()
    mocked_container_manager.create.assert_not_called()


PULL_FAILURE = ImagePullError(
    "lhjnilsson/foreverbull:latest",
    'Error response from daemon: Get "https://registry-1.docker.io/v2/": dial tcp: i/o timeout',
)


def test_create_stops_on_failed_image_pull(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.verify_images.side_effect = PULL_FAILURE
    result = runner(cli.cli, "create")
    assert result.exit_code == 1
    lines = result.stdout.splitlines()
    assert [line for line in lines if line.startswith("✖")] == [
        "✖ Couldn't download the Foreverbull image lhjnilsson/foreverbull:latest: Docker could not reach the registry."
    ]
    assert "Check your internet connection and run `fbenv create` again." in result.stdout
    assert "Nothing was changed." in result.stdout
    assert "Run `fbenv create --verbose` to see Docker's error." in result.stdout
    assert "i/o timeout" not in result.stdout
    assert "Traceback" not in result.stdout
    assert result.exception is None or isinstance(result.exception, SystemExit)
    mocked_container_manager.create.assert_not_called()


def test_create_verbose_shows_docker_error(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.verify_images.side_effect = PULL_FAILURE
    result = runner(cli.cli, ["create", "--verbose"])
    assert result.exit_code == 1
    assert "Docker said: Error response from daemon" in result.stdout
    assert "i/o timeout" in result.stdout
    assert "Traceback" not in result.stdout


def test_update_stops_on_failed_image_pull(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.update.side_effect = PULL_FAILURE
    result = runner(cli.cli, "update")
    assert result.exit_code == 1
    assert "✖ Couldn't download the Foreverbull image" in result.stdout
    assert "run `fbenv update` again." in result.stdout
    assert "Nothing was changed." in result.stdout
    assert "Run `fbenv update --verbose` to see Docker's error." in result.stdout
    assert "Traceback" not in result.stdout


def _reject_version(manager: MagicMock) -> None:
    type(manager.config).version = PropertyMock(side_effect=VersionError("latest", "1.2.3"))


def test_create_rejects_a_version_other_than_local(runner, mocked_container_manager: MagicMock):
    _reject_version(mocked_container_manager)
    result = runner(cli.cli, "create")
    assert result.exit_code == 1
    lines = result.stdout.splitlines()
    assert [line for line in lines if line.startswith("✖")] == ["✖ VERSION can only be `local`, got `latest`."]
    assert "installed client version (1.2.3)" in result.stdout
    assert "Nothing was changed." in result.stdout
    assert "Traceback" not in result.stdout
    mocked_container_manager.preflight.assert_not_called()
    mocked_container_manager.verify_images.assert_not_called()
    mocked_container_manager.create.assert_not_called()


def test_update_rejects_a_version_other_than_local(runner, mocked_container_manager: MagicMock):
    _reject_version(mocked_container_manager)
    result = runner(cli.cli, "update")
    assert result.exit_code == 1
    assert "✖ VERSION can only be `local`, got `latest`." in result.stdout
    assert "Nothing was changed." in result.stdout
    mocked_container_manager.update.assert_not_called()


def test_start_stops_on_failed_host_check(runner, mocked_container_manager: MagicMock):
    mocked_container_manager.preflight.side_effect = PORT_FAILURE
    result = runner(cli.cli, "start")
    assert result.exit_code == 1
    assert "✖ Port" in result.stdout
    mocked_container_manager.start.assert_not_called()


def test_create(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "create")
    assert result.exit_code == 0
    mocked_container_manager.verify_images.assert_called_once()
    mocked_container_manager.preflight.assert_called_once_with()
    mocked_container_manager.create.assert_called_once_with()
    mocked_container_manager.start.assert_not_called()
    assert "fbenv start" in result.stdout
    assert "Host checks" not in result.stdout


def test_create_start(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "create --start")
    assert result.exit_code == 0
    mocked_container_manager.create.assert_called_once_with()
    mocked_container_manager.start.assert_called_once()


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


def test_stop_remove_continues_when_scale_fails(runner, mocked_container_manager: MagicMock):
    completed = MagicMock(returncode=1, stdout="", stderr="port 0")
    mocked_container_manager.stop.side_effect = CommandError(["helm", "upgrade"], completed)
    result = runner(cli.cli, "stop --remove --purge")
    assert result.exit_code == 0
    mocked_container_manager.remove.assert_called_once_with(True)


def test_stop_scale_failure_is_an_error(runner, mocked_container_manager: MagicMock):
    completed = MagicMock(returncode=1, stdout="", stderr="port 0")
    mocked_container_manager.stop.side_effect = CommandError(["helm", "upgrade"], completed)
    result = runner(cli.cli, "stop")
    assert result.exit_code != 0
    mocked_container_manager.remove.assert_not_called()


def test_update(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, "update")
    assert result.exit_code == 0
    mocked_container_manager.update.assert_called_once()


def test_create_cluster_flag(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, ["create", "--cluster", "remote", "--context", "prod"])
    assert result.exit_code == 0
    mocked_container_manager.create.assert_called_once_with()
    mocked_container_manager.start.assert_not_called()
    mocked_container_manager.config.apply_cli_overrides.assert_called()
    kwargs = mocked_container_manager.config.apply_cli_overrides.call_args.kwargs
    assert kwargs["cluster"] == "remote"
    assert kwargs["context"] == "prod"


def test_stop_and_status_forward_cluster_flags(runner, mocked_container_manager: MagicMock):
    result = runner(cli.cli, ["stop", "--cluster", "remote", "--namespace", "fb"])
    assert result.exit_code == 0
    mocked_container_manager.stop.assert_called_once()
    kwargs = mocked_container_manager.config.apply_cli_overrides.call_args.kwargs
    assert kwargs["cluster"] == "remote"
    assert kwargs["namespace"] == "fb"

    mocked_container_manager.config.apply_cli_overrides.reset_mock()
    result = runner(cli.cli, ["status", "--cluster", "local", "--namespace", "custom"])
    assert result.exit_code == 0
    mocked_container_manager.status_rows.assert_called()
    kwargs = mocked_container_manager.config.apply_cli_overrides.call_args.kwargs
    assert kwargs["cluster"] == "local"
    assert kwargs["namespace"] == "custom"


def _isolated_project(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("HOME", str(home))
    monkeypatch.delenv("FOREVERBULL_CONFIG", raising=False)
    monkeypatch.delenv("FOREVERBULL_BROKER_ADDRESS", raising=False)
    return home


def test_use_shows_current_cluster(tmp_path, monkeypatch):
    _isolated_project(tmp_path, monkeypatch)
    result = CliRunner().invoke(cli.cli, ["use"])
    assert result.exit_code == 0
    assert "cluster: local" in result.stdout
    assert "broker: localhost:50055" in result.stdout


def test_use_local_writes_config(tmp_path, monkeypatch):
    _isolated_project(tmp_path, monkeypatch)
    result = CliRunner().invoke(cli.cli, ["use", "local"])
    assert result.exit_code == 0, result.output
    config_path = tmp_path / ".foreverbull" / "config.yaml"
    assert config_path.is_file()
    assert "cluster: local" in config_path.read_text()
    assert "Wrote" in result.stdout


def test_use_remote_writes_config(tmp_path, monkeypatch):
    _isolated_project(tmp_path, monkeypatch)
    result = CliRunner().invoke(
        cli.cli,
        [
            "use",
            "remote",
            "--kubeconfig",
            "/tmp/kube",
            "--context",
            "prod",
            "--namespace",
            "fb",
            "--broker-address",
            "broker.example:50055",
        ],
    )
    assert result.exit_code == 0, result.output
    text = (tmp_path / ".foreverbull" / "config.yaml").read_text()
    assert "cluster: remote" in text
    assert "context: prod" in text
    assert "broker_address: broker.example:50055" in text
    assert "cluster: remote" in result.stdout
    assert "broker: broker.example:50055" in result.stdout


def test_use_remote_requires_broker_address(tmp_path, monkeypatch):
    _isolated_project(tmp_path, monkeypatch)
    result = CliRunner().invoke(cli.cli, ["use", "remote", "--context", "prod"])
    assert result.exit_code != 0
    assert "broker_address" in (result.output or "") + str(result.exception)
