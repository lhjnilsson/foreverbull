import json
import subprocess

from collections.abc import Mapping
from collections.abc import Sequence
from pathlib import Path

import pytest

from foreverbull_cli.env.environment import BROKER_ERROR_ADVICE
from foreverbull_cli.env.environment import NO_WORKLOAD
from foreverbull_cli.env.environment import WORKLOADS
from foreverbull_cli.env.environment import ClusterManager
from foreverbull_cli.env.environment import CommandError
from foreverbull_cli.env.environment import CommandRunner
from foreverbull_cli.env.environment import Config
from foreverbull_cli.env.environment import Environment
from foreverbull_cli.env.environment import ImagePullError
from foreverbull_cli.env.environment import MissingToolError
from foreverbull_cli.env.environment import WorkloadStatus
from foreverbull_cli.env.environment import classify_pull_error
from foreverbull_cli.env.environment import load_cluster_config
from foreverbull_cli.env.environment import match_broker_error
from foreverbull_cli.env.preflight import CheckState
from foreverbull_cli.env.preflight import PreflightError


pytestmark = pytest.mark.usefixtures("fb_env")


def _completed(stdout: str = "", stderr: str = "", returncode: int = 0) -> subprocess.CompletedProcess[str]:
    return subprocess.CompletedProcess(args=[], returncode=returncode, stdout=stdout, stderr=stderr)


class FakeRunner(CommandRunner):
    def __init__(self, tools: set[str] | None = None):
        self.tools = tools if tools is not None else {"kind", "helm", "kubectl", "docker"}
        self.calls: list[list[str]] = []
        self.stdins: list[str | None] = []
        self.handlers: list[tuple] = []
        self.default = _completed()

    def which(self, name: str) -> str | None:
        return f"/usr/bin/{name}" if name in self.tools else None

    def on(
        self,
        prefix: list[str],
        result: subprocess.CompletedProcess[str] | Exception,
        *,
        contains: str | None = None,
    ) -> None:
        self.handlers.append((prefix, contains, result))

    def run(
        self,
        args: Sequence[str],
        *,
        check: bool = True,
        env: Mapping[str, str] | None = None,
        timeout: float | None = None,
        stdin: str | None = None,
    ) -> subprocess.CompletedProcess[str]:
        command = [str(a) for a in args]
        self.calls.append(command)
        self.stdins.append(stdin)
        result: subprocess.CompletedProcess[str] | Exception = self.default
        for prefix, contains, handler in reversed(self.handlers):
            if command[: len(prefix)] == prefix and (contains is None or contains in command):
                result = handler
                break
        if isinstance(result, Exception):
            raise result
        if check and result.returncode != 0:
            raise CommandError(command, result)
        return result

    def calls_starting_with(self, *prefix: str) -> list[list[str]]:
        wanted = list(prefix)
        return [call for call in self.calls if call[: len(wanted)] == wanted]


def _write_repo(tmp_path: Path) -> Path:
    chart = tmp_path / "deploy" / "helm" / "foreverbull"
    chart.mkdir(parents=True)
    (chart / "Chart.yaml").write_text("name: foreverbull\n")
    kind = tmp_path / "deploy" / "kind"
    kind.mkdir(parents=True)
    (kind / "cluster.yaml").write_text("kind: Cluster\n")
    return tmp_path


def _manager(
    tmp_path: Path,
    runner: FakeRunner,
    config: Config | None = None,
    environ: dict[str, str] | None = None,
    ready_timeout_seconds: int = 5,
) -> ClusterManager:
    repo = _write_repo(tmp_path)
    if config is None:
        config = Config(load_files=False, environ=environ or {})
    environment = Environment(path=str(tmp_path / "project"))
    return ClusterManager(
        environment,
        config,
        runner=runner,
        repo_root=repo,
        ready_timeout_seconds=ready_timeout_seconds,
    )


def _trading_secret_calls(runner: FakeRunner) -> list[list[str]]:
    return [call for call in runner.calls if "foreverbull-trading" in call]


def _trading_secret_index(runner: FakeRunner) -> int:
    return next(i for i, call in enumerate(runner.calls) if "foreverbull-trading" in call)


def _assert_secret_not_shown(runner: FakeRunner, secret: str, capsys: pytest.CaptureFixture[str]) -> None:
    blob = "\n".join(" ".join(call) for call in runner.calls)
    assert secret not in blob
    assert all(secret not in (stdin or "") for stdin in runner.stdins)
    captured = capsys.readouterr()
    assert secret not in captured.out
    assert secret not in captured.err


def _pod(
    name: str,
    app: str,
    image: str,
    phase: str = "Running",
    ready: bool = True,
    container_id: str = "containerd://abcdef1234567890",
    terminating: bool = False,
) -> dict:
    metadata: dict = {"name": f"{app}-abc", "labels": {"app.kubernetes.io/name": app}}
    if terminating:
        metadata["deletionTimestamp"] = "2026-10-16T21:00:00Z"
    return {
        "kind": "Pod",
        "metadata": metadata,
        "spec": {"containers": [{"image": image}]},
        "status": {
            "phase": phase,
            "containerStatuses": [
                {"ready": ready, "image": image, "containerID": container_id},
            ],
        },
    }


def _deploy(name: str, image: str, replicas: int = 1) -> dict:
    return {
        "kind": "Deployment",
        "metadata": {"name": name},
        "spec": {
            "replicas": replicas,
            "template": {"spec": {"containers": [{"image": image}]}},
        },
    }


class TestConfig:
    def test_version_default(self):
        assert Config(load_files=False, environ={}).version == "latest"

    def test_version_from_env(self):
        assert Config(load_files=False, environ={"VERSION": "local"}).version == "local"

    def test_default_cluster_is_local(self):
        uut = Config(load_files=False, environ={})
        assert uut.cluster == "local"
        assert uut.is_local is True
        assert uut.namespace == "foreverbull"
        assert uut.kind_name == "foreverbull"
        assert uut.kind_config == "deploy/kind/cluster.yaml"
        assert uut.kube_context == "kind-foreverbull"
        assert uut.broker_address == "localhost:50055"

    def test_foreverbull_image(self):
        assert Config(load_files=False, environ={}).foreverbull_image == "lhjnilsson/foreverbull:latest"

    def test_backtest_image(self):
        assert Config(load_files=False, environ={}).zipline_image == "lhjnilsson/zipline:latest"

    def test_grafana_image(self):
        assert Config(load_files=False, environ={}).grafana_image == "lhjnilsson/fb-grafana:latest"

    def test_remote_namespace_and_kube_flags(self):
        uut = Config(
            {
                "cluster": "remote",
                "namespace": "foreverbull",
                "remote": {
                    "kubeconfig": "/tmp/kube",
                    "context": "prod",
                    "namespace": "fb",
                    "broker_address": "broker.example:50055",
                },
            },
            load_files=False,
            environ={},
        )
        assert uut.is_local is False
        assert uut.namespace == "fb"
        assert uut.kubeconfig == "/tmp/kube"
        assert uut.kube_context == "prod"
        assert uut.broker_address == "broker.example:50055"

    def test_rejects_unknown_cluster(self):
        with pytest.raises(ValueError, match="local"):
            Config({"cluster": "kind"}, load_files=False, environ={})

    def test_broker_address_env_overrides_local_and_remote(self):
        local = Config(load_files=False, environ={"FOREVERBULL_BROKER_ADDRESS": "from-env:9"})
        assert local.broker_address == "from-env:9"
        remote = Config(
            {
                "cluster": "remote",
                "remote": {"broker_address": "from-file:1"},
            },
            load_files=False,
            environ={"FOREVERBULL_BROKER_ADDRESS": "from-env:9"},
        )
        assert remote.broker_address == "from-env:9"

    def test_cli_overrides_switch_to_remote(self):
        uut = Config(load_files=False, environ={})
        uut.apply_cli_overrides(
            cluster="remote",
            kubeconfig="/tmp/kube",
            context="prod",
            namespace="fb",
            broker_address="broker.example:50055",
        )
        assert uut.cluster == "remote"
        assert uut.is_local is False
        assert uut.kubeconfig == "/tmp/kube"
        assert uut.kube_context == "prod"
        assert uut.namespace == "fb"
        assert uut.broker_address == "broker.example:50055"

    def test_cli_namespace_override_local(self):
        uut = Config(load_files=False, environ={})
        uut.apply_cli_overrides(namespace="custom")
        assert uut.is_local is True
        assert uut.namespace == "custom"

    def test_persist_selection_writes_project_config(self, tmp_path: Path):
        cwd = tmp_path / "proj"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        uut = Config(load_files=True, cwd=cwd, home=home, environ={})
        path = uut.persist_selection(
            "remote",
            kubeconfig="/tmp/kube",
            context="prod",
            namespace="fb",
            broker_address="broker.example:50055",
        )
        assert path == cwd / ".foreverbull" / "config.yaml"
        assert uut.cluster == "remote"
        assert uut.kube_context == "prod"
        assert uut.namespace == "fb"
        assert uut.broker_address == "broker.example:50055"
        written = path.read_text()
        assert "cluster: remote" in written
        assert "broker_address: broker.example:50055" in written

    def test_persist_remote_requires_broker_address(self, tmp_path: Path):
        uut = Config(load_files=False, cwd=tmp_path, home=tmp_path, environ={})
        with pytest.raises(ValueError, match="remote.broker_address"):
            uut.persist_selection("remote", context="prod")


class TestLoadClusterConfig:
    def test_merge_order_later_wins(self, tmp_path: Path):
        home = tmp_path / "home"
        cwd = tmp_path / "cwd"
        (home / ".foreverbull").mkdir(parents=True)
        (cwd / ".foreverbull").mkdir(parents=True)
        (home / ".foreverbull" / "config.yaml").write_text("cluster: remote\nnamespace: from-user\n")
        (cwd / ".foreverbull" / "config.yaml").write_text("namespace: from-project\nkind:\n  name: custom\n")
        explicit = tmp_path / "explicit.yaml"
        explicit.write_text("kind:\n  name: explicit\n")
        merged = load_cluster_config(
            cwd=cwd,
            home=home,
            environ={"FOREVERBULL_CONFIG": str(explicit)},
        )
        assert merged["cluster"] == "remote"
        assert merged["namespace"] == "from-project"
        assert merged["kind"]["name"] == "explicit"
        assert merged["kind"]["config"] == "deploy/kind/cluster.yaml"

    def test_missing_explicit_config(self, tmp_path: Path):
        with pytest.raises(FileNotFoundError, match="FOREVERBULL_CONFIG"):
            load_cluster_config(
                cwd=tmp_path, home=tmp_path, environ={"FOREVERBULL_CONFIG": str(tmp_path / "nope.yaml")}
            )


class TestEnvironment:
    def test_path(self, tmp_path: Path):
        uut = Environment(path=str(tmp_path))
        assert uut.path == tmp_path / ".foreverbull"

    def test_purge_keeps_config_yaml(self, tmp_path: Path):
        uut = Environment(path=str(tmp_path))
        (uut.path / "config.yaml").write_text("cluster: local\n")
        (uut.path / "postgres").mkdir()
        (uut.path / "other.txt").write_text("x")
        uut.purge()
        assert (uut.path / "config.yaml").is_file()
        assert not (uut.path / "postgres").exists()
        assert not (uut.path / "other.txt").exists()


class TestClusterManager:
    def test_missing_tools(self, tmp_path: Path):
        runner = FakeRunner(tools=set())
        uut = _manager(tmp_path, runner)
        with pytest.raises(MissingToolError, match="kind is not installed"):
            uut.create()

    def test_create_local_kind_helm_and_load(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout=""))
        uut = _manager(tmp_path, runner, environ={"VERSION": "latest"})
        uut.create()
        assert runner.calls_starting_with("kind", "create", "cluster")
        create_at = next(i for i, call in enumerate(runner.calls) if call[:3] == ["kind", "create", "cluster"])
        load_at = next(i for i, call in enumerate(runner.calls) if call[:3] == ["kind", "load", "docker-image"])
        assert create_at < load_at
        kind_create = runner.calls_starting_with("kind", "create", "cluster")[0]
        assert "--name" in kind_create
        assert kind_create[kind_create.index("--name") + 1] == "foreverbull"
        assert runner.calls_starting_with("kind", "load", "docker-image")
        helm = runner.calls_starting_with("helm", "upgrade", "--install")
        assert len(helm) == 1
        assert "--set" in helm[0]
        assert "global.imageTag=latest" in helm[0]
        assert "expose.hostPort=true" in helm[0]
        assert "broker.logLevel=debug" in helm[0]
        assert "--kube-context" in helm[0]
        assert helm[0][helm[0].index("--kube-context") + 1] == "kind-foreverbull"
        assert "--namespace" in helm[0]
        assert helm[0][helm[0].index("--namespace") + 1] == "foreverbull"
        assert "replicaCount=0" in helm[0]

    def test_create_skips_kind_create_when_cluster_exists(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        uut = _manager(tmp_path, runner)
        uut.create()
        assert runner.calls_starting_with("kind", "create", "cluster") == []
        assert runner.calls_starting_with("helm", "upgrade", "--install")

    def test_create_uses_cli_overrides_for_remote(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.config.apply_cli_overrides(
            cluster="remote",
            kubeconfig="/tmp/kube",
            context="prod",
            namespace="fb",
        )
        uut.create()
        assert runner.calls_starting_with("kind") == []
        helm = runner.calls_starting_with("helm", "upgrade", "--install")[0]
        assert helm[helm.index("--kubeconfig") + 1] == "/tmp/kube"
        assert helm[helm.index("--kube-context") + 1] == "prod"
        assert helm[helm.index("--namespace") + 1] == "fb"
        assert "expose.hostPort=false" in helm

    def test_create_remote_skips_kind(self, tmp_path: Path):
        runner = FakeRunner()
        config = Config(
            {
                "cluster": "remote",
                "remote": {"kubeconfig": "/tmp/kube", "context": "prod", "namespace": "fb"},
            },
            load_files=False,
            environ={"VERSION": "latest"},
        )
        uut = _manager(tmp_path, runner, config=config)
        uut.create()
        assert runner.calls_starting_with("kind") == []
        helm = runner.calls_starting_with("helm", "upgrade", "--install")[0]
        assert helm[helm.index("--kubeconfig") + 1] == "/tmp/kube"
        assert helm[helm.index("--kube-context") + 1] == "prod"
        assert helm[helm.index("--namespace") + 1] == "fb"
        assert "expose.hostPort=false" in helm
        assert "broker.logLevel=debug" not in helm

    def test_load_images_errors_when_local_tag_missing(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["docker", "image", "inspect"], _completed(returncode=1, stderr="not found"))
        uut = _manager(tmp_path, runner, environ={"VERSION": "local"})
        with pytest.raises(RuntimeError, match="Missing local images"):
            uut.load_images()

    def test_load_images_pulls_missing_non_local_tags(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["docker", "image", "inspect"], _completed(returncode=1, stderr="not found"))
        uut = _manager(tmp_path, runner, environ={"VERSION": "latest"})
        uut.load_images()
        assert runner.calls_starting_with("docker", "pull")
        assert runner.calls_starting_with("kind", "load", "docker-image")

    def test_verify_images_raises_image_pull_error_on_failed_pull(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["docker", "image", "inspect"], _completed(returncode=1, stderr="No such image"))
        runner.on(
            ["docker", "pull"],
            _completed(
                returncode=1,
                stderr='Error response from daemon: Get "https://registry-1.docker.io/v2/": '
                "dial tcp: lookup registry-1.docker.io: no such host",
            ),
        )
        uut = _manager(tmp_path, runner, environ={"VERSION": "latest"})
        with pytest.raises(ImagePullError) as raised:
            uut.verify_images()
        assert raised.value.image == "lhjnilsson/foreverbull:latest"
        assert raised.value.cause == "network"
        assert "no such host" in raised.value.detail
        assert "lhjnilsson/foreverbull:latest" in str(raised.value)
        assert "no such host" not in str(raised.value)
        assert len(runner.calls_starting_with("docker", "pull")) == 1
        assert runner.calls_starting_with("kind", "create") == []
        assert runner.calls_starting_with("kind", "load", "docker-image") == []

    def test_start_scales_and_waits(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.start()
        helm = runner.calls_starting_with("helm", "upgrade")[0]
        assert "--install" not in helm
        assert "--reuse-values" in helm
        assert "replicaCount=1" in helm
        assert helm[helm.index("--kube-context") + 1] == "kind-foreverbull"
        wait = runner.calls_starting_with("kubectl")[0]
        assert wait[:2] == ["kubectl", "--context"]
        assert wait[2] == "kind-foreverbull"
        assert "wait" in wait
        assert "--for=condition=available" in wait
        assert "deployment/foreverbull" in wait

    def test_start_times_out_if_unhealthy(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(
            ["kubectl"],
            _completed(returncode=1, stderr="timed out waiting for the condition"),
            contains="wait",
        )
        uut = _manager(tmp_path, runner, ready_timeout_seconds=1)
        with pytest.raises(TimeoutError, match="Timed out waiting for healthy"):
            uut.start()

    def test_stop_scales_to_zero(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.stop()
        helm = runner.calls_starting_with("helm", "upgrade")[0]
        assert "--reuse-values" in helm
        assert "replicaCount=0" in helm
        assert runner.calls_starting_with("kubectl") == []

    def test_stop_missing_release_is_ok(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["helm", "upgrade"], _completed(returncode=1, stderr="Error: release: not found"))
        uut = _manager(tmp_path, runner)
        uut.stop()

    def test_remove_uninstalls_helm(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.remove()
        uninstall = runner.calls_starting_with("helm", "uninstall")[0]
        assert "foreverbull" in uninstall
        assert runner.calls_starting_with("kind", "delete") == []

    def test_remove_purge_deletes_kind_and_data(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        uut = _manager(tmp_path, runner)
        data_dir = uut.environment.path / "postgres"
        data_dir.mkdir()
        (data_dir / "x").write_text("data")
        (uut.environment.path / "config.yaml").write_text("cluster: local\n")
        uut.remove(purge=True)
        assert runner.calls_starting_with("helm", "uninstall")
        assert runner.calls_starting_with("kind", "delete", "cluster")
        assert not data_dir.exists()
        assert (uut.environment.path / "config.yaml").is_file()

    def test_remove_purge_remote_skips_kind_delete(self, tmp_path: Path):
        runner = FakeRunner()
        config = Config({"cluster": "remote"}, load_files=False, environ={})
        uut = _manager(tmp_path, runner, config=config)
        uut.remove(purge=True)
        assert runner.calls_starting_with("kind") == []

    def test_create_applies_trading_profiles_before_helm(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        uut = _manager(tmp_path, runner)
        secret = "apiSecret: do-not-print-this"
        path = uut.environment.path / "trading-profiles.yaml"
        path.write_text(secret + "\n")
        uut.create()
        secret_at = _trading_secret_index(runner)
        helm_at = next(i for i, call in enumerate(runner.calls) if call[:3] == ["helm", "upgrade", "--install"])
        assert secret_at < helm_at
        create_cmd = runner.calls[secret_at]
        assert create_cmd[create_cmd.index("create") + 1 : create_cmd.index("create") + 4] == [
            "secret",
            "generic",
            "foreverbull-trading",
        ]
        assert f"--from-file=profiles.yaml={path}" in create_cmd
        assert "--dry-run=client" in create_cmd
        apply_at = next(i for i, call in enumerate(runner.calls) if call[-3:] == ["apply", "-f", "-"])
        assert secret_at < apply_at < helm_at
        assert runner.stdins[apply_at] == ""
        _assert_secret_not_shown(runner, secret, capsys)

    def test_create_skips_trading_profiles_when_file_absent(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        uut = _manager(tmp_path, runner)
        assert not (uut.environment.path / "trading-profiles.yaml").exists()
        uut.create()
        assert _trading_secret_calls(runner) == []
        assert runner.calls_starting_with("helm", "upgrade", "--install")

    def test_create_trading_profiles_remote_flags(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]):
        runner = FakeRunner()
        config = Config(
            {
                "cluster": "remote",
                "remote": {"kubeconfig": "/tmp/kube", "context": "prod", "namespace": "fb"},
            },
            load_files=False,
            environ={},
        )
        uut = _manager(tmp_path, runner, config=config)
        secret = "password: do-not-print-this"
        path = uut.environment.path / "trading-profiles.yaml"
        path.write_text(secret + "\n")
        runner.on(
            ["kubectl"],
            _completed(returncode=1, stderr='Error from server (NotFound): namespaces "fb" not found'),
            contains="get",
        )
        uut.create()
        create_ns = runner.calls_starting_with("kubectl")
        assert any(
            call[call.index("create") + 1 :][:2] == ["namespace", "fb"] for call in create_ns if "create" in call
        )
        secret_cmd = runner.calls[_trading_secret_index(runner)]
        assert secret_cmd[secret_cmd.index("--kubeconfig") + 1] == "/tmp/kube"
        assert secret_cmd[secret_cmd.index("--context") + 1] == "prod"
        assert secret_cmd[secret_cmd.index("--namespace") + 1] == "fb"
        assert f"--from-file=profiles.yaml={path}" in secret_cmd
        helm_at = next(i for i, call in enumerate(runner.calls) if call[:3] == ["helm", "upgrade", "--install"])
        assert _trading_secret_index(runner) < helm_at
        _assert_secret_not_shown(runner, secret, capsys)

    def test_create_trading_profiles_kubectl_failure_skips_helm(
        self, tmp_path: Path, capsys: pytest.CaptureFixture[str]
    ):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        runner.on(
            ["kubectl"],
            _completed(returncode=1, stderr="secret failed"),
            contains="foreverbull-trading",
        )
        uut = _manager(tmp_path, runner)
        secret = "accessToken: do-not-print-this"
        (uut.environment.path / "trading-profiles.yaml").write_text(secret + "\n")
        with pytest.raises(CommandError, match="foreverbull-trading"):
            uut.create()
        assert runner.calls_starting_with("helm", "upgrade") == []
        _assert_secret_not_shown(runner, secret, capsys)

    def test_update_applies_trading_profiles_before_helm(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner, environ={"VERSION": "local"})
        secret = "apiKey: do-not-print-this"
        path = uut.environment.path / "trading-profiles.yaml"
        path.write_text(secret + "\n")
        uut.update()
        secret_at = _trading_secret_index(runner)
        helm_at = next(i for i, call in enumerate(runner.calls) if call[:3] == ["helm", "upgrade", "--install"])
        assert secret_at < helm_at
        assert f"--from-file=profiles.yaml={path}" in runner.calls[secret_at]
        assert "--reuse-values" in runner.calls[helm_at]
        _assert_secret_not_shown(runner, secret, capsys)

    def test_update_skips_trading_profiles_when_file_absent(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner, environ={"VERSION": "local"})
        uut.update()
        assert _trading_secret_calls(runner) == []
        assert runner.calls_starting_with("helm", "upgrade", "--install")

    def test_update_loads_and_helm_upgrades(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner, environ={"VERSION": "local"})
        uut.update()
        assert runner.calls_starting_with("kind", "load", "docker-image")
        helm = runner.calls_starting_with("helm", "upgrade", "--install")[0]
        assert "--reuse-values" in helm
        assert "global.imageTag=local" in helm
        assert "replicaCount=" not in " ".join(helm)

    def test_status_from_kubectl_json(self, tmp_path: Path):
        items = [
            _deploy("postgres", "postgres:13.3-alpine"),
            _pod("postgres-abc", "postgres", "postgres:13.3-alpine"),
            _deploy("nats", "nats:2.10-alpine", replicas=0),
            _deploy("minio", "minio/minio:latest"),
            _pod("minio-abc", "minio", "minio/minio:latest", phase="Pending", ready=False),
            _deploy("zipline", "lhjnilsson/zipline:latest"),
            _pod("zipline-abc", "zipline", "lhjnilsson/zipline:latest"),
            _deploy("foreverbull", "lhjnilsson/foreverbull:latest"),
            _pod("foreverbull-abc", "foreverbull", "lhjnilsson/foreverbull:latest"),
            _deploy("grafana", "lhjnilsson/fb-grafana:latest"),
            _pod(
                "grafana-abc",
                "grafana",
                "lhjnilsson/fb-grafana:latest",
                phase="Failed",
                ready=False,
            ),
        ]
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": items})))
        uut = _manager(tmp_path, runner)
        rows = {row.name: row for row in uut.status_rows()}
        assert rows["postgres"].status == WorkloadStatus.RUNNING
        assert rows["postgres"].image == "postgres:13.3-alpine"
        assert rows["postgres"].container_id == "abcdef123456"
        assert rows["nats"].status == WorkloadStatus.STOPPED
        assert (rows["nats"].image, rows["nats"].container_id) == (NO_WORKLOAD, NO_WORKLOAD)
        assert rows["minio"].status == WorkloadStatus.PENDING
        assert rows["grafana"].status == WorkloadStatus.FAILED
        assert [row.service for row in uut.status_rows()] == [
            "Postgres",
            "NATS",
            "Minio",
            "Zipline",
            "Foreverbull",
            "Grafana",
        ]

    def test_status_cluster_unreachable_is_not_found(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(returncode=1, stderr="The connection to the server was refused"))
        uut = _manager(tmp_path, runner)
        rows = uut.status_rows()
        assert all(row.status == WorkloadStatus.NOT_FOUND for row in rows)
        assert len(rows) == 6

    def test_status_right_after_stop_is_never_running(self, tmp_path: Path):
        # Right after `fbenv stop` the Deployments want 0 replicas, but their pods
        # still report Running and ready while Kubernetes terminates them.
        names = [name for _, name in WORKLOADS]
        after_stop = []
        for name in names:
            after_stop.append(_deploy(name, f"{name}:latest", replicas=0))
            after_stop.append(_pod(f"{name}-abc", name, f"{name}:latest", terminating=name != "nats"))
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": after_stop})))
        uut = _manager(tmp_path, runner)
        uut.stop()
        rows = uut.status_rows()
        assert len(rows) == len(names)
        assert {row.status for row in rows} == {WorkloadStatus.STOPPING}
        assert all(row.image == NO_WORKLOAD and row.container_id == NO_WORKLOAD for row in rows)

        runner.on(
            ["kubectl"],
            _completed(stdout=json.dumps({"items": [_deploy(name, f"{name}:latest", replicas=0) for name in names]})),
        )
        rows = uut.status_rows()
        assert {row.status for row in rows} == {WorkloadStatus.STOPPED}
        assert all(row.image == NO_WORKLOAD and row.container_id == NO_WORKLOAD for row in rows)

    def test_status_keeps_not_found_for_missing_service(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": [_deploy("postgres", "postgres:13.3-alpine")]})))
        rows = {row.name: row for row in _manager(tmp_path, runner).status_rows()}
        assert rows["postgres"].status == WorkloadStatus.PENDING
        assert rows["postgres"].container_id == "not found"
        assert rows["nats"].status == WorkloadStatus.NOT_FOUND
        assert (rows["nats"].image, rows["nats"].container_id) == ("not found", "not found")

    def test_status_ignores_terminating_pod_while_starting(self, tmp_path: Path):
        items = [
            _deploy("postgres", "postgres:13.3-alpine"),
            _pod("postgres-old", "postgres", "postgres:13.3-alpine", terminating=True),
            _pod("postgres-new", "postgres", "postgres:13.3-alpine", phase="Pending", ready=False),
            _deploy("nats", "nats:2.10-alpine"),
            _pod("nats-old", "nats", "nats:2.10-alpine", terminating=True),
        ]
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": items})))
        uut = _manager(tmp_path, runner)
        rows = {row.name: row for row in uut.status_rows()}
        assert rows["postgres"].status == WorkloadStatus.PENDING
        assert rows["nats"].status == WorkloadStatus.PENDING

    def test_status_queries_cluster_after_stop(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(
            ["kubectl"],
            _completed(
                stdout=json.dumps(
                    {
                        "items": [
                            _deploy("postgres", "postgres:13.3-alpine"),
                            _pod("postgres-abc", "postgres", "postgres:13.3-alpine"),
                        ]
                    }
                )
            ),
        )
        uut = _manager(tmp_path, runner)
        assert uut.postgres.status() == WorkloadStatus.RUNNING
        runner.on(
            ["kubectl"],
            _completed(stdout=json.dumps({"items": [_deploy("postgres", "postgres:13.3-alpine", replicas=0)]})),
        )
        uut.stop()
        assert uut.postgres.status() == WorkloadStatus.STOPPED

    def test_workload_accessors_use_status_cache(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(
            ["kubectl"],
            _completed(
                stdout=json.dumps(
                    {
                        "items": [
                            _deploy("postgres", "postgres:13.3-alpine"),
                            _pod("postgres-abc", "postgres", "postgres:13.3-alpine"),
                        ]
                    }
                )
            ),
        )
        uut = _manager(tmp_path, runner)
        assert uut.postgres.status() == WorkloadStatus.RUNNING
        assert uut.postgres.image_version() == "postgres:13.3-alpine"
        assert uut.postgres.container_id() == "abcdef123456"
        assert uut.postgres.name == "postgres"

    def test_verify_images_checks_docker_without_kind_load(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.verify_images()
        assert runner.calls_starting_with("docker", "image", "inspect")
        assert runner.calls_starting_with("kind", "load", "docker-image") == []

    def test_verify_images_requires_tools(self, tmp_path: Path):
        runner = FakeRunner(tools=set())
        uut = _manager(tmp_path, runner)
        with pytest.raises(MissingToolError, match="kind is not installed"):
            uut.verify_images()

    def test_missing_kind_config(self, tmp_path: Path):
        runner = FakeRunner()
        repo = tmp_path / "empty"
        repo.mkdir()
        (repo / "deploy" / "helm" / "foreverbull").mkdir(parents=True)
        (repo / "deploy" / "helm" / "foreverbull" / "Chart.yaml").write_text("name: foreverbull\n")
        uut = ClusterManager(
            Environment(path=str(tmp_path / "project")),
            Config(load_files=False, environ={}),
            runner=runner,
            repo_root=repo,
        )
        with pytest.raises(FileNotFoundError, match="Kind config not found"):
            uut.ensure_kind_cluster()

    def test_create_stop_status_local_vs_remote(self, tmp_path: Path):
        local_runner = FakeRunner()
        local_runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        local_runner.on(["kubectl"], _completed(stdout=json.dumps({"items": []})))
        local = _manager(tmp_path, local_runner)
        local.create()
        local.stop()
        rows = local.status_rows()
        assert local_runner.calls_starting_with("kind", "load")
        assert local_runner.calls_starting_with("helm", "upgrade", "--install")
        assert any(
            "--reuse-values" in call and "replicaCount=0" in call
            for call in local_runner.calls_starting_with("helm", "upgrade")
        )
        assert len(rows) == 6
        assert all(row.status == WorkloadStatus.NOT_FOUND for row in rows)

        remote_runner = FakeRunner()
        remote_runner.on(["kubectl"], _completed(stdout=json.dumps({"items": []})))
        remote = ClusterManager(
            local.environment,
            Config(
                {
                    "cluster": "remote",
                    "remote": {
                        "kubeconfig": "/tmp/kube",
                        "context": "prod",
                        "namespace": "fb",
                        "broker_address": "broker.example:50055",
                    },
                },
                load_files=False,
                environ={},
            ),
            runner=remote_runner,
            repo_root=local.repo_root,
            ready_timeout_seconds=5,
        )
        remote.create()
        remote.stop()
        remote.status_rows()
        assert remote_runner.calls_starting_with("kind") == []
        helm = remote_runner.calls_starting_with("helm", "upgrade", "--install")[0]
        assert helm[helm.index("--kube-context") + 1] == "prod"
        assert helm[helm.index("--namespace") + 1] == "fb"
        kubectl = remote_runner.calls_starting_with("kubectl")
        assert kubectl[0][kubectl[0].index("--context") + 1] == "prod"


def _write_kind_ports(repo: Path, ports: list[int]) -> None:
    mappings = "".join(f"      - containerPort: {port}\n        hostPort: {port}\n" for port in ports)
    (repo / "deploy" / "kind" / "cluster.yaml").write_text(
        f"kind: Cluster\nnodes:\n  - role: control-plane\n    extraPortMappings:\n{mappings}"
    )


def _healthy_docker(runner: FakeRunner) -> None:
    runner.on(["docker", "info"], _completed(stdout="27.3.1 8589934592\n"))


class TestHostChecks:
    def test_local_checks_all_pass(self, tmp_path: Path):
        runner = FakeRunner()
        _healthy_docker(runner)
        uut = _manager(tmp_path, runner)
        _write_kind_ports(tmp_path, [5432, 3000])
        uut.port_probe = lambda port: True
        checks = uut.host_checks()
        assert [check.name for check in checks] == [
            "kind",
            "helm",
            "kubectl",
            "docker",
            "Docker daemon",
            "Docker memory",
            "Host ports",
        ]
        assert all(check.state == CheckState.PASS for check in checks)
        uut.preflight()

    def test_missing_tool_is_the_first_failure(self, tmp_path: Path):
        runner = FakeRunner(tools={"helm", "kubectl", "docker"})
        _healthy_docker(runner)
        uut = _manager(tmp_path, runner)
        uut.port_probe = lambda port: True
        with pytest.raises(PreflightError, match="✖ kind is not installed.") as exc:
            uut.preflight()
        assert "https://kind.sigs.k8s.io" in str(exc.value)

    def test_stopped_docker_aborts_before_any_cluster_change(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["docker", "info"], _completed(returncode=1, stderr="Cannot connect to the Docker daemon"))
        uut = _manager(tmp_path, runner)
        uut.port_probe = lambda port: True
        with pytest.raises(PreflightError, match="✖ Docker is not running."):
            uut.preflight()
        assert runner.calls_starting_with("kind", "create") == []
        assert runner.calls_starting_with("kind", "get") == []
        assert runner.calls_starting_with("helm", "upgrade") == []
        memory = next(check for check in uut.host_checks() if check.name == "Docker memory")
        assert memory.state == CheckState.SKIP

    def test_docker_info_timeout_is_a_failed_check(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["docker", "info"], subprocess.TimeoutExpired(["docker", "info"], 15))
        uut = _manager(tmp_path, runner)
        uut.port_probe = lambda port: True
        daemon = next(check for check in uut.host_checks() if check.name == "Docker daemon")
        assert daemon.state == CheckState.FAIL
        assert "did not answer in 15 s" in daemon.detail

    def test_busy_port_aborts_with_the_port_and_service(self, tmp_path: Path):
        runner = FakeRunner()
        _healthy_docker(runner)
        uut = _manager(tmp_path, runner)
        _write_kind_ports(tmp_path, [5432, 4222])
        uut.port_probe = lambda port: port != 5432
        with pytest.raises(PreflightError, match=r"Port 5432 \(Postgres\) is already in use."):
            uut.preflight()

    def test_ports_held_by_running_cluster_pass_without_probing(self, tmp_path: Path):
        runner = FakeRunner()
        _healthy_docker(runner)
        runner.on(["docker", "inspect"], _completed(stdout="true\n"), contains="foreverbull-control-plane")
        uut = _manager(tmp_path, runner)
        _write_kind_ports(tmp_path, [5432])

        def never(port: int) -> bool:
            raise AssertionError(f"probed {port}")

        uut.port_probe = never
        ports = next(check for check in uut.host_checks() if check.name == "Host ports")
        assert ports.state == CheckState.PASS
        assert "held by the foreverbull Kind cluster" in ports.detail

    def test_stopped_cluster_still_probes_ports(self, tmp_path: Path):
        runner = FakeRunner()
        _healthy_docker(runner)
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        runner.on(["docker", "inspect"], _completed(stdout="false\n"), contains="foreverbull-control-plane")
        uut = _manager(tmp_path, runner)
        _write_kind_ports(tmp_path, [5432])
        uut.port_probe = lambda port: port != 5432
        ports = next(check for check in uut.host_checks() if check.name == "Host ports")
        assert ports.state == CheckState.FAIL
        assert "Port 5432 (Postgres) is already in use." in ports.detail

    def test_missing_kind_config_fails_the_ports_check(self, tmp_path: Path):
        runner = FakeRunner()
        _healthy_docker(runner)
        uut = _manager(tmp_path, runner)
        (tmp_path / "deploy" / "kind" / "cluster.yaml").unlink()
        ports = next(check for check in uut.host_checks() if check.name == "Host ports")
        assert ports.state == CheckState.FAIL
        assert "Kind config not found" in ports.detail

    def test_missing_docker_skips_the_daemon_checks(self, tmp_path: Path):
        runner = FakeRunner(tools={"kind", "helm", "kubectl"})
        uut = _manager(tmp_path, runner)
        uut.port_probe = lambda port: True
        checks = {check.name: check for check in uut.host_checks()}
        assert checks["docker"].state == CheckState.FAIL
        assert checks["Docker daemon"].state == CheckState.SKIP
        assert checks["Docker memory"].state == CheckState.SKIP
        assert runner.calls_starting_with("docker") == []
        with pytest.raises(PreflightError, match="docker is not installed."):
            uut.preflight()

    def test_remote_checks_only_helm_and_kubectl(self, tmp_path: Path):
        runner = FakeRunner(tools={"helm", "kubectl"})
        config = Config(
            {"cluster": "remote", "remote": {"broker_address": "fb.example:50055"}},
            load_files=False,
            environ={},
        )
        uut = _manager(tmp_path, runner, config=config)
        checks = {check.name: check.state for check in uut.host_checks()}
        assert checks == {
            "helm": CheckState.PASS,
            "kubectl": CheckState.PASS,
            "Docker daemon": CheckState.SKIP,
            "Docker memory": CheckState.SKIP,
            "Host ports": CheckState.SKIP,
        }
        assert runner.calls_starting_with("docker") == []
        uut.preflight()


def _broker_items(phase: str, ready: bool) -> list[dict]:
    return [
        _deploy("foreverbull", "lhjnilsson/foreverbull:latest"),
        _pod("foreverbull-abc", "foreverbull", "lhjnilsson/foreverbull:latest", phase=phase, ready=ready),
    ]


class TestBrokerDiagnosis:
    POSTGRES_ERROR = (
        "failed to setup: could not reach postgres at postgres:5432 after 20 attempts; "
        "check POSTGRES_URL and that Postgres is running"
    )

    def test_running_broker_has_no_diagnosis(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": _broker_items("Running", True)})))
        uut = _manager(tmp_path, runner)
        assert uut.broker_diagnosis() is None
        assert [call for call in runner.calls if "logs" in call] == []

    def test_crashing_broker_explains_unreachable_database(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": _broker_items("Running", False)})))
        runner.on(
            ["kubectl"], _completed(returncode=1, stderr="previous terminated container not found"), contains="logs"
        )
        runner.on(["kubectl"], _completed(stdout=f"{self.POSTGRES_ERROR}\n"), contains="--previous")
        uut = _manager(tmp_path, runner)
        diagnosis = uut.broker_diagnosis()
        assert diagnosis is not None
        assert diagnosis.advice.startswith("The broker can't reach its database.")
        assert diagnosis.detail == self.POSTGRES_ERROR
        logs = [call for call in runner.calls if "logs" in call]
        assert all("--tail=20" in call for call in logs)

    def test_restarted_broker_with_unknown_error_shows_its_last_lines(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": _broker_items("Running", False)})))
        runner.on(["kubectl"], _completed(stdout="starting\n"), contains="logs")
        runner.on(["kubectl"], _completed(stdout="boom: something new\n"), contains="--previous")
        uut = _manager(tmp_path, runner)
        diagnosis = uut.broker_diagnosis()
        assert diagnosis is not None
        assert diagnosis.advice.startswith("The broker stopped with an error.")
        assert diagnosis.detail == "boom: something new"

    def test_starting_broker_without_errors_has_no_diagnosis(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": _broker_items("Pending", False)})))
        runner.on(
            ["kubectl"],
            _completed(stdout="postgres at postgres:5432 is not reachable (attempt 1 of 20)\n"),
            contains="logs",
        )
        runner.on(["kubectl"], _completed(returncode=1, stderr="not found"), contains="--previous")
        uut = _manager(tmp_path, runner)
        assert uut.broker_diagnosis() is None

    def test_start_timeout_leads_with_the_broker_advice(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(stdout=json.dumps({"items": _broker_items("Running", False)})))
        runner.on(["kubectl"], _completed(stdout=f"{self.POSTGRES_ERROR}\n"), contains="logs")
        runner.on(["kubectl"], _completed(returncode=1, stderr="timed out waiting for the condition"), contains="wait")
        uut = _manager(tmp_path, runner, ready_timeout_seconds=1)
        with pytest.raises(TimeoutError) as exc:
            uut.start()
        lines = str(exc.value).splitlines()
        assert lines[0].startswith("The broker can't reach its database.")
        assert lines[1] == self.POSTGRES_ERROR
        assert lines[2].startswith("Timed out waiting for healthy")


class TestBrokerErrors:
    def test_unreachable_postgres_gets_a_plain_sentence_and_the_log_line(self):
        logs = (
            "postgres at postgres:5432 is not reachable (attempt 19 of 20), retrying in 3s\n"
            "failed to setup: could not reach postgres at postgres:5432 after 20 attempts; "
            "check POSTGRES_URL and that Postgres is running: dial tcp: connection refused\n"
        )
        diagnosis = match_broker_error(logs)
        assert diagnosis is not None
        assert diagnosis.advice == BROKER_ERROR_ADVICE[0][1]
        assert diagnosis.advice.startswith("The broker can't reach its database.")
        assert diagnosis.detail.startswith("failed to setup: could not reach postgres at postgres:5432")

    def test_no_free_port_gets_a_plain_sentence(self):
        logs = "error: no free port in the backtest port range (27000-27015): stop other sessions"
        diagnosis = match_broker_error(logs)
        assert diagnosis is not None
        assert diagnosis.advice.startswith("The broker has no free backtest port.")

    def test_retry_lines_alone_are_not_an_error(self):
        logs = "postgres at postgres:5432 is not reachable (attempt 1 of 20), retrying in 3s"
        assert match_broker_error(logs) is None


class TestImagePullError:
    @pytest.mark.parametrize(
        "output,cause",
        [
            (
                "Error response from daemon: manifest for lhjnilsson/foreverbull:v9 not found: manifest unknown",
                "missing tag",
            ),
            (
                (
                    "Error response from daemon: pull access denied for lhjnilsson/foreverbul, "
                    "repository does not exist or may require 'docker login'"
                ),
                "registry",
            ),
            ("toomanyrequests: You have reached your pull rate limit.", "registry"),
            (
                (
                    "no matching manifest for linux/arm64/v8 in the manifest list entries: "
                    "manifest for lhjnilsson/zipline:latest"
                ),
                "platform",
            ),
            (
                (
                    "permission denied while trying to connect to the Docker daemon socket at "
                    "unix:///var/run/docker.sock: Post /v1.24/images/create: dial unix: connect: permission denied"
                ),
                "daemon",
            ),
            (
                "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
                "daemon",
            ),
            ("open /root/.docker/config.json: permission denied", "unknown"),
            ("Get https://registry-1.docker.io/v2/: net/http: TLS handshake timeout", "network"),
            ("dial tcp 54.1.2.3:443: connect: connection refused", "network"),
            ("something odd happened", "unknown"),
        ],
    )
    def test_classify(self, output: str, cause: str):
        assert classify_pull_error(output) == cause

    def test_hint_names_the_command(self):
        exc = ImagePullError("lhjnilsson/zipline:latest", "dial tcp: i/o timeout")
        assert exc.hint("fbenv create") == "Check your internet connection and run `fbenv create` again."

    def test_missing_tag_hint_points_at_version(self):
        exc = ImagePullError("lhjnilsson/zipline:v9", "manifest unknown")
        assert "VERSION" in exc.hint("fbenv create")

    def test_platform_hint_names_the_architecture(self):
        exc = ImagePullError(
            "lhjnilsson/zipline:latest", "no matching manifest for linux/arm64/v8 in the manifest list entries"
        )
        assert exc.hint("fbenv create") == (
            "This image has no build for your machine (arm64). "
            "Set the VERSION environment variable to a release that supports arm64, "
            "or report it at https://github.com/lhjnilsson/foreverbull/issues."
        )

    def test_platform_hint_uses_the_detected_architecture(self):
        exc = ImagePullError("lhjnilsson/zipline:latest", "no matching manifest for linux/riscv64 in the manifest list")
        assert exc.hint("fbenv create").startswith("This image has no build for your machine (riscv64). ")
        assert "a release that supports riscv64," in exc.hint("fbenv create")

    def test_daemon_hint_says_start_docker(self):
        exc = ImagePullError("lhjnilsson/zipline:latest", "Cannot connect to the Docker daemon at unix:///x")
        assert exc.hint("fbenv update") == (
            "Start Docker, or fix your permissions on its socket, then run `fbenv update` again."
        )

    def test_registry_hint_is_wait_and_retry(self):
        exc = ImagePullError("lhjnilsson/zipline:latest", "toomanyrequests")
        assert exc.hint("fbenv update") == "Wait a few minutes and run `fbenv update` again."

    def test_missing_tag_hint_names_the_environment_variable(self):
        exc = ImagePullError("lhjnilsson/zipline:v9", "manifest unknown")
        assert "the VERSION environment variable" in exc.hint("fbenv create")


def _second_pull_fails(tmp_path: Path) -> tuple[ClusterManager, FakeRunner, list[str]]:
    runner = FakeRunner()
    runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
    runner.on(["docker", "image", "inspect"], _completed(returncode=1))
    uut = _manager(tmp_path, runner)
    images = list(uut.config.app_images)
    runner.on(["docker", "pull"], _completed(returncode=1, stderr="dial tcp: i/o timeout"), contains=images[1])
    return uut, runner, images


class TestPullFailureChangesNothing:
    MUTATING = (["kind", "load"], ["kind", "create"], ["helm"], ["kubectl"])

    def _mutations(self, runner: FakeRunner) -> list[list[str]]:
        return [c for c in runner.calls if any(c[: len(p)] == p for p in self.MUTATING)]

    def test_update_second_pull_failure_skips_kind_load_and_helm(self, tmp_path: Path):
        uut, runner, images = _second_pull_fails(tmp_path)
        with pytest.raises(ImagePullError) as exc:
            uut.update()
        assert exc.value.image == images[1]
        pulls = [c[2] for c in runner.calls if c[:2] == ["docker", "pull"]]
        assert pulls == images[:2]
        assert self._mutations(runner) == []

    def test_create_second_pull_failure_skips_cluster_and_helm(self, tmp_path: Path):
        uut, runner, _images = _second_pull_fails(tmp_path)
        with pytest.raises(ImagePullError):
            uut.verify_images()
        assert self._mutations(runner) == []

    def test_update_pulls_every_image_before_kind_load(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["docker", "image", "inspect"], _completed(returncode=1))
        uut = _manager(tmp_path, runner)
        uut.update()
        first_load = next(i for i, c in enumerate(runner.calls) if c[:2] == ["kind", "load"])
        last_pull = max(i for i, c in enumerate(runner.calls) if c[:2] == ["docker", "pull"])
        first_helm = next(i for i, c in enumerate(runner.calls) if c[:1] == ["helm"])
        assert last_pull < first_load < first_helm
