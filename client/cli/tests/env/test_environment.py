import json
import subprocess

from collections.abc import Mapping
from collections.abc import Sequence
from pathlib import Path

import pytest

from foreverbull_cli.env.environment import CommandError
from foreverbull_cli.env.environment import CommandRunner
from foreverbull_cli.env.environment import Config
from foreverbull_cli.env.environment import ContainerManager
from foreverbull_cli.env.environment import ContainerStatus
from foreverbull_cli.env.environment import Environment
from foreverbull_cli.env.environment import MissingToolError
from foreverbull_cli.env.environment import load_cluster_config


pytestmark = pytest.mark.usefixtures("fb_env")


def _completed(stdout: str = "", stderr: str = "", returncode: int = 0) -> subprocess.CompletedProcess[str]:
    return subprocess.CompletedProcess(args=[], returncode=returncode, stdout=stdout, stderr=stderr)


class FakeRunner(CommandRunner):
    def __init__(self, tools: set[str] | None = None):
        self.tools = tools if tools is not None else {"kind", "helm", "kubectl", "docker"}
        self.calls: list[list[str]] = []
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
    ) -> subprocess.CompletedProcess[str]:
        command = [str(a) for a in args]
        self.calls.append(command)
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
) -> ContainerManager:
    repo = _write_repo(tmp_path)
    if config is None:
        config = Config(load_files=False, environ=environ or {})
    environment = Environment(path=str(tmp_path / "project"))
    return ContainerManager(
        environment,
        config,
        runner=runner,
        repo_root=repo,
        ready_timeout_seconds=ready_timeout_seconds,
    )


def _pod(
    name: str,
    app: str,
    image: str,
    phase: str = "Running",
    ready: bool = True,
    container_id: str = "containerd://abcdef1234567890",
) -> dict:
    return {
        "kind": "Pod",
        "metadata": {"name": f"{app}-abc", "labels": {"app.kubernetes.io/name": app}},
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

    def test_postgres_image(self):
        assert Config(load_files=False, environ={}).postgres_image == "postgres:13.3-alpine"

    def test_minio_image(self):
        assert Config(load_files=False, environ={}).minio_image == "minio/minio:latest"

    def test_nats_image(self):
        assert Config(load_files=False, environ={}).nats_image == "nats:2.10-alpine"

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
    def test_locations(self, tmp_path: Path):
        uut = Environment(path=str(tmp_path))
        assert uut.postgres_location == tmp_path / ".foreverbull" / "postgres"
        assert uut.minio_location == tmp_path / ".foreverbull" / "minio"
        assert uut.nats_location == tmp_path / ".foreverbull" / "nats"

    def test_purge_keeps_config_yaml(self, tmp_path: Path):
        uut = Environment(path=str(tmp_path))
        (uut.path / "config.yaml").write_text("cluster: local\n")
        (uut.path / "postgres").mkdir()
        (uut.path / "other.txt").write_text("x")
        uut.purge()
        assert (uut.path / "config.yaml").is_file()
        assert not (uut.path / "postgres").exists()
        assert not (uut.path / "other.txt").exists()


class TestContainerManager:
    def test_missing_tools(self, tmp_path: Path):
        runner = FakeRunner(tools=set())
        uut = _manager(tmp_path, runner)
        with pytest.raises(MissingToolError, match="kind is not installed"):
            uut.create()

    def test_create_local_kind_helm_and_load(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout=""))
        uut = _manager(tmp_path, runner, environ={"VERSION": "latest"})
        uut.create(start=True)
        assert runner.calls_starting_with("kind", "create", "cluster")
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
        assert "replicaCount=0" not in helm[0]

    def test_create_skips_kind_create_when_cluster_exists(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        uut = _manager(tmp_path, runner)
        uut.create()
        assert runner.calls_starting_with("kind", "create", "cluster") == []
        assert runner.calls_starting_with("helm", "upgrade", "--install")

    def test_create_no_start_sets_replica_count_zero(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kind", "get", "clusters"], _completed(stdout="foreverbull\n"))
        uut = _manager(tmp_path, runner)
        uut.create(start=False)
        helm = runner.calls_starting_with("helm", "upgrade", "--install")[0]
        assert "replicaCount=0" in helm

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

    def test_start_scales_and_waits(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.start()
        scale = runner.calls_starting_with("kubectl")[0]
        assert scale[:2] == ["kubectl", "--context"]
        assert scale[2] == "kind-foreverbull"
        assert "scale" in scale
        assert "--replicas=1" in scale
        assert "postgres" in scale
        wait = runner.calls_starting_with("kubectl")[1]
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
        scale = runner.calls_starting_with("kubectl")[0]
        assert "scale" in scale
        assert "--replicas=0" in scale

    def test_stop_missing_deployments_is_ok(self, tmp_path: Path):
        runner = FakeRunner()
        runner.on(["kubectl"], _completed(returncode=1, stderr='deployments.apps "postgres" not found'))
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
        data_dir = uut.environment.postgres_location
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

    def test_update_loads_and_helm_upgrades(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner, environ={"VERSION": "local"})
        uut.update()
        assert runner.calls_starting_with("kind", "load", "docker-image")
        helm = runner.calls_starting_with("helm", "upgrade", "--install")[0]
        assert "--reuse-values" in helm
        assert "global.imageTag=local" in helm

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
        assert rows["postgres"].status == ContainerStatus.RUNNING
        assert rows["postgres"].image == "postgres:13.3-alpine"
        assert rows["postgres"].container_id == "abcdef123456"
        assert rows["nats"].status == ContainerStatus.STOPPED
        assert rows["nats"].image == "nats:2.10-alpine"
        assert rows["minio"].status == ContainerStatus.PENDING
        assert rows["grafana"].status == ContainerStatus.FAILED
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
        assert all(row.status == ContainerStatus.NOT_FOUND for row in rows)
        assert len(rows) == 6

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
        assert uut.postgres.status() == ContainerStatus.RUNNING
        assert uut.postgres.image_version() == "postgres:13.3-alpine"
        assert uut.postgres.container_id() == "abcdef123456"
        assert uut.postgres.name == "postgres"

    def test_verify_images_requires_tools(self, tmp_path: Path):
        runner = FakeRunner()
        uut = _manager(tmp_path, runner)
        uut.verify_images()
        assert runner.calls_starting_with("kind", "load", "docker-image")

    def test_missing_kind_config(self, tmp_path: Path):
        runner = FakeRunner()
        repo = tmp_path / "empty"
        repo.mkdir()
        (repo / "deploy" / "helm" / "foreverbull").mkdir(parents=True)
        (repo / "deploy" / "helm" / "foreverbull" / "Chart.yaml").write_text("name: foreverbull\n")
        uut = ContainerManager(
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
        assert any("scale" in call and "--replicas=0" in call for call in local_runner.calls_starting_with("kubectl"))
        assert len(rows) == 6
        assert all(row.status == ContainerStatus.NOT_FOUND for row in rows)

        remote_runner = FakeRunner()
        remote_runner.on(["kubectl"], _completed(stdout=json.dumps({"items": []})))
        remote = ContainerManager(
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
