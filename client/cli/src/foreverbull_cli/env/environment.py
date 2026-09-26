import copy
import json
import os
import shutil
import subprocess

from collections.abc import Mapping
from collections.abc import Sequence
from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
from typing import Any

from foreverbull.broker.config import BROKER_ADDRESS_ENV
from foreverbull.broker.config import DEFAULT_BROKER_ADDRESS
from foreverbull.broker.config import DEFAULT_CLUSTER_CONFIG
from foreverbull.broker.config import DEFAULT_KIND_CONFIG
from foreverbull.broker.config import DEFAULT_KIND_NAME
from foreverbull.broker.config import DEFAULT_NAMESPACE
from foreverbull.broker.config import deep_merge
from foreverbull.broker.config import load_cluster_config
from foreverbull.broker.config import project_config_path
from foreverbull.broker.config import save_cluster_config


RELEASE_NAME = "foreverbull"
CHART_RELATIVE_PATH = Path("deploy") / "helm" / "foreverbull"

TOOL_HINTS = {
    "kind": "https://kind.sigs.k8s.io/docs/user/quick-start/",
    "helm": "https://helm.sh/docs/intro/install/",
    "kubectl": "https://kubernetes.io/docs/tasks/tools/",
    "docker": "https://docs.docker.com/get-docker/",
}

WORKLOADS: tuple[tuple[str, str], ...] = (
    ("Postgres", "postgres"),
    ("NATS", "nats"),
    ("Minio", "minio"),
    ("Zipline", "zipline"),
    ("Foreverbull", "foreverbull"),
    ("Grafana", "grafana"),
)
DEPLOYMENTS: tuple[str, ...] = tuple(name for _, name in WORKLOADS)


class MissingToolError(RuntimeError):
    def __init__(self, names: str | Sequence[str]):
        if isinstance(names, str):
            names = [names]
        parts = []
        for name in names:
            hint = TOOL_HINTS.get(name)
            if hint:
                parts.append(f"{name} is not installed. {hint}")
            else:
                parts.append(f"{name} is not installed.")
        super().__init__("\n".join(parts))
        self.names = list(names)


class CommandError(RuntimeError):
    def __init__(self, args: Sequence[str], completed: subprocess.CompletedProcess[str]):
        cmd = " ".join(str(a) for a in args)
        err = (completed.stderr or completed.stdout or "").strip()
        message = f"{cmd} failed (exit {completed.returncode})"
        if err:
            message = f"{message}: {err}"
        super().__init__(message)
        self.cmd = list(args)
        self.completed = completed


class CommandRunner:
    def which(self, name: str) -> str | None:
        return shutil.which(name)

    def run(
        self,
        args: Sequence[str],
        *,
        check: bool = True,
        env: Mapping[str, str] | None = None,
        timeout: float | None = None,
    ) -> subprocess.CompletedProcess[str]:
        command = [str(a) for a in args]
        try:
            completed = subprocess.run(
                command,
                check=False,
                capture_output=True,
                text=True,
                env=None if env is None else {**os.environ, **env},
                timeout=timeout,
            )
        except FileNotFoundError as exc:
            raise MissingToolError(command[0]) from exc
        if check and completed.returncode != 0:
            raise CommandError(command, completed)
        return completed


class WorkloadStatus(StrEnum):
    NOT_FOUND = "not_found"
    RUNNING = "running"
    STOPPED = "stopped"
    PENDING = "pending"
    FAILED = "failed"


@dataclass(frozen=True)
class WorkloadRow:
    service: str
    name: str
    status: str
    image: str
    container_id: str


def find_repo_root(start: Path | None = None) -> Path:
    seen: set[Path] = set()
    bases = []
    if start is not None:
        bases.append(start)
    bases.append(Path.cwd())
    bases.append(Path(__file__).resolve())
    for base in bases:
        for path in [base, *base.parents]:
            if path in seen:
                continue
            seen.add(path)
            if (path / CHART_RELATIVE_PATH / "Chart.yaml").is_file():
                return path
    raise FileNotFoundError("Could not find Foreverbull repository root (expected deploy/helm/foreverbull).")


class Config:
    def __init__(
        self,
        data: dict[str, Any] | None = None,
        *,
        environ: Mapping[str, str] | None = None,
        load_files: bool = True,
        cwd: Path | None = None,
        home: Path | None = None,
        cluster: str | None = None,
        kubeconfig: str | None = None,
        context: str | None = None,
        namespace: str | None = None,
        broker_address: str | None = None,
    ):
        self._environ = os.environ if environ is None else environ
        self._cwd = Path.cwd() if cwd is None else cwd
        self._home = Path.home() if home is None else home
        self._load_files = load_files
        self._broker_address_override: str | None = None
        if data is None and load_files:
            self._data = load_cluster_config(cwd=self._cwd, home=self._home, environ=self._environ)
        elif data is None:
            self._data = copy.deepcopy(DEFAULT_CLUSTER_CONFIG)
        elif load_files:
            self._data = deep_merge(load_cluster_config(cwd=self._cwd, home=self._home, environ=self._environ), data)
        else:
            self._data = deep_merge(copy.deepcopy(DEFAULT_CLUSTER_CONFIG), data)

        self._normalize_cluster()
        self.apply_cli_overrides(
            cluster=cluster,
            kubeconfig=kubeconfig,
            context=context,
            namespace=namespace,
            broker_address=broker_address,
        )

    def _normalize_cluster(self) -> None:
        cluster = str(self._data.get("cluster") or "local").lower()
        if cluster not in {"local", "remote"}:
            raise ValueError(f"cluster must be 'local' or 'remote', got {cluster!r}")
        self._data["cluster"] = cluster

    def apply_cli_overrides(
        self,
        *,
        cluster: str | None = None,
        kubeconfig: str | None = None,
        context: str | None = None,
        namespace: str | None = None,
        broker_address: str | None = None,
    ) -> None:
        if cluster is not None:
            self._data["cluster"] = cluster
            self._normalize_cluster()
        remote = dict(self._data.get("remote") or {})
        if kubeconfig is not None:
            remote["kubeconfig"] = kubeconfig
        if context is not None:
            remote["context"] = context
        if broker_address is not None:
            remote["broker_address"] = broker_address
            self._broker_address_override = broker_address
        if namespace is not None:
            if self.is_local:
                self._data["namespace"] = namespace
            else:
                remote["namespace"] = namespace
        self._data["remote"] = remote

    def persist_selection(
        self,
        cluster: str,
        *,
        kubeconfig: str | None = None,
        context: str | None = None,
        namespace: str | None = None,
        broker_address: str | None = None,
    ) -> Path:
        cluster = cluster.lower()
        if cluster not in {"local", "remote"}:
            raise ValueError(f"cluster must be 'local' or 'remote', got {cluster!r}")
        updates: dict[str, Any] = {"cluster": cluster}
        remote_updates: dict[str, Any] = {}
        if kubeconfig is not None:
            remote_updates["kubeconfig"] = kubeconfig
        if context is not None:
            remote_updates["context"] = context
        if broker_address is not None:
            remote_updates["broker_address"] = broker_address
        if namespace is not None:
            if cluster == "local":
                updates["namespace"] = namespace
            else:
                remote_updates["namespace"] = namespace
        if remote_updates:
            updates["remote"] = remote_updates
        preview = Config(
            updates,
            environ=self._environ,
            load_files=self._load_files,
            cwd=self._cwd,
            home=self._home,
        )
        if not preview.is_local and not preview.broker_address:
            raise ValueError(
                "remote.broker_address is required; pass --broker-address or set it in .foreverbull/config.yaml"
            )
        path = save_cluster_config(updates, cwd=self._cwd)
        self._load_files = True
        self._data = load_cluster_config(cwd=self._cwd, home=self._home, environ=self._environ)
        self._normalize_cluster()
        self._broker_address_override = None
        return path

    @property
    def version(self) -> str:
        return self._environ.get("VERSION", "latest")

    @property
    def cluster(self) -> str:
        return str(self._data.get("cluster") or "local")

    @property
    def is_local(self) -> bool:
        return self.cluster == "local"

    @property
    def namespace(self) -> str:
        if not self.is_local:
            remote_ns = str((self._data.get("remote") or {}).get("namespace") or "").strip()
            if remote_ns:
                return remote_ns
        return str(self._data.get("namespace") or DEFAULT_NAMESPACE)

    @property
    def release_name(self) -> str:
        return RELEASE_NAME

    @property
    def kind_name(self) -> str:
        return str((self._data.get("kind") or {}).get("name") or DEFAULT_KIND_NAME)

    @property
    def kind_config(self) -> str:
        return str((self._data.get("kind") or {}).get("config") or DEFAULT_KIND_CONFIG)

    @property
    def kubeconfig(self) -> str:
        if self.is_local:
            return ""
        return str((self._data.get("remote") or {}).get("kubeconfig") or "").strip()

    @property
    def kube_context(self) -> str:
        if self.is_local:
            return f"kind-{self.kind_name}"
        return str((self._data.get("remote") or {}).get("context") or "").strip()

    @property
    def broker_address(self) -> str:
        env_addr = str(self._environ.get(BROKER_ADDRESS_ENV) or "").strip()
        if env_addr:
            return env_addr
        if self._broker_address_override:
            return self._broker_address_override
        if self.is_local:
            return DEFAULT_BROKER_ADDRESS
        return str((self._data.get("remote") or {}).get("broker_address") or "").strip()

    @property
    def config_path(self) -> Path:
        return project_config_path(self._cwd)

    @property
    def foreverbull_image(self) -> str:
        return f"lhjnilsson/foreverbull:{self.version}"

    @property
    def zipline_image(self) -> str:
        return f"lhjnilsson/zipline:{self.version}"

    @property
    def grafana_image(self) -> str:
        return f"lhjnilsson/fb-grafana:{self.version}"

    @property
    def app_images(self) -> tuple[str, str, str]:
        return self.foreverbull_image, self.zipline_image, self.grafana_image


class Environment:
    def __init__(self, path: str | None = None):
        if path is None:
            self.path = Path(".") / ".foreverbull"
        else:
            self.path = Path(path) / ".foreverbull"
        self.path.mkdir(parents=True, exist_ok=True)

    def purge(self) -> None:
        if not self.path.exists():
            return
        for child in self.path.iterdir():
            if child.name == "config.yaml":
                continue
            if child.is_dir():
                shutil.rmtree(child)
            else:
                child.unlink()


class Workload:
    def __init__(self, service: str, name: str, manager: "ClusterManager"):
        self.service = service
        self.name = name
        self._manager = manager

    def status(self) -> str:
        return self._manager._row(self.name).status

    def image_version(self) -> str:
        return self._manager._row(self.name).image

    def container_id(self) -> str:
        return self._manager._row(self.name).container_id


class ClusterManager:
    def __init__(
        self,
        environment: Environment,
        config: Config,
        runner: CommandRunner | None = None,
        repo_root: Path | None = None,
        ready_timeout_seconds: int = 120,
    ):
        self.config = config
        self.environment = environment
        self.runner = runner or CommandRunner()
        self._repo_root = repo_root
        self.ready_timeout_seconds = ready_timeout_seconds
        self._status_cache: dict[str, WorkloadRow] | None = None
        self.postgres = Workload("Postgres", "postgres", self)
        self.nats = Workload("NATS", "nats", self)
        self.minio = Workload("Minio", "minio", self)
        self.zipline = Workload("Zipline", "zipline", self)
        self.foreverbull = Workload("Foreverbull", "foreverbull", self)
        self.grafana = Workload("Grafana", "grafana", self)
        self.workloads = [
            self.postgres,
            self.nats,
            self.minio,
            self.zipline,
            self.foreverbull,
            self.grafana,
        ]

    @property
    def repo_root(self) -> Path:
        if self._repo_root is None:
            self._repo_root = find_repo_root()
        return self._repo_root

    @property
    def chart_path(self) -> Path:
        return self.repo_root / CHART_RELATIVE_PATH

    @property
    def kind_config_path(self) -> Path:
        raw = Path(self.config.kind_config)
        if raw.is_absolute():
            return raw
        return self.repo_root / raw

    def require_tools(self, *names: str) -> None:
        missing = [name for name in names if not self.runner.which(name)]
        if missing:
            raise MissingToolError(missing)

    def _kube_flags(self, *, helm: bool = False) -> list[str]:
        flags: list[str] = []
        if self.config.kubeconfig:
            flags.extend(["--kubeconfig", self.config.kubeconfig])
        context = self.config.kube_context
        if context:
            flags.extend(["--kube-context" if helm else "--context", context])
        flags.extend(["--namespace", self.config.namespace])
        return flags

    def _tools_for_create(self) -> tuple[str, ...]:
        if self.config.is_local:
            return ("kind", "helm", "kubectl")
        return ("helm", "kubectl")

    def verify_images(self) -> None:
        """Confirm app images exist locally. Kind load waits until the cluster exists."""
        self.require_tools(*self._tools_for_create())
        if self.config.is_local:
            self._local_images()

    def create(self) -> None:
        """Install the cluster and chart with workloads stopped."""
        self.require_tools(*self._tools_for_create())
        if self.config.is_local:
            self.ensure_kind_cluster()
            self.load_images()
        self.helm_upgrade(replicas=0)

    def start(self) -> None:
        self.require_tools("helm", "kubectl")
        self._set_replicas(1)
        self._wait_until_ready()

    def stop(self) -> None:
        self._set_replicas(0, missing_ok=True)

    def remove(self, purge: bool = False) -> None:
        self.require_tools("helm")
        self._helm_uninstall()
        if purge:
            if self.config.is_local:
                self.require_tools("kind")
                self._kind_delete_cluster()
            self.environment.purge()
        self._status_cache = None

    def update(self) -> None:
        self.require_tools(*self._tools_for_create())
        if self.config.is_local:
            self.load_images()
        self.helm_upgrade(reuse_values=True)
        self._status_cache = None

    def status_rows(self) -> list[WorkloadRow]:
        self._status_cache = None
        return [self._row(name) for _, name in WORKLOADS]

    def _row(self, name: str) -> WorkloadRow:
        if self._status_cache is None:
            self._status_cache = self._query_status()
        return self._status_cache[name]

    def ensure_kind_cluster(self) -> None:
        self.require_tools("kind")
        config_path = self.kind_config_path
        if not config_path.is_file():
            raise FileNotFoundError(f"Kind config not found: {config_path}")
        if self._kind_cluster_exists():
            return
        self.runner.run(
            [
                "kind",
                "create",
                "cluster",
                "--name",
                self.config.kind_name,
                "--config",
                str(config_path),
            ]
        )

    def _local_images(self) -> list[str]:
        """Return images present in the local Docker daemon. Pull non-local tags that are missing."""
        images = list(self.config.app_images)
        present: list[str] = []
        missing: list[str] = []
        docker = self.runner.which("docker")
        if docker:
            for image in images:
                inspect = self.runner.run(["docker", "image", "inspect", image], check=False)
                if inspect.returncode == 0:
                    present.append(image)
                else:
                    missing.append(image)
        else:
            missing = list(images)

        if missing and self.config.version == "local":
            lines = ["Missing local images:"]
            lines.extend(f"  {image}" for image in missing)
            lines.append("Build them first:")
            lines.append("  task build:docker")
            lines.append("  task client:docker:zipline")
            lines.append("  task grafana:build:docker")
            if not docker:
                raise MissingToolError("docker")
            raise RuntimeError("\n".join(lines))

        if missing and docker and self.config.version != "local":
            for image in missing:
                self.runner.run(["docker", "pull", image])
                present.append(image)

        return present

    def load_images(self) -> None:
        if not self.config.is_local:
            return
        self.require_tools("kind")
        present = self._local_images()
        if present:
            self.runner.run(["kind", "load", "docker-image", *present, "--name", self.config.kind_name])

    def helm_upgrade(self, *, replicas: int | None = None, reuse_values: bool = False) -> None:
        self.require_tools("helm")
        chart = self.chart_path
        if not chart.is_dir():
            raise FileNotFoundError(f"Helm chart not found: {chart}")
        cmd = [
            "helm",
            "upgrade",
            "--install",
            self.config.release_name,
            str(chart),
            *self._kube_flags(helm=True),
            "--create-namespace",
            "--set",
            f"global.imageTag={self.config.version}",
            "--set",
            f"expose.hostPort={'true' if self.config.is_local else 'false'}",
        ]
        if self.config.is_local:
            cmd.extend(["--set", "broker.logLevel=debug"])
        if replicas is not None:
            cmd.extend(["--set", f"replicaCount={replicas}"])
        if reuse_values:
            cmd.append("--reuse-values")
        self.runner.run(cmd)
        self._status_cache = None

    def _set_replicas(self, replicas: int, *, missing_ok: bool = False) -> None:
        """Record the replica count on the Helm release so later upgrades keep it."""
        self.require_tools("helm")
        chart = self.chart_path
        if not chart.is_dir():
            raise FileNotFoundError(f"Helm chart not found: {chart}")
        cmd = [
            "helm",
            "upgrade",
            self.config.release_name,
            str(chart),
            *self._kube_flags(helm=True),
            "--reuse-values",
            "--set",
            f"replicaCount={replicas}",
        ]
        result = self.runner.run(cmd, check=not missing_ok)
        if missing_ok and result.returncode != 0:
            combined = f"{result.stdout} {result.stderr}".lower()
            if "not found" in combined:
                return
            raise CommandError(cmd, result)
        self._status_cache = None

    def _kind_cluster_exists(self) -> bool:
        result = self.runner.run(["kind", "get", "clusters"], check=False)
        if result.returncode != 0:
            raise CommandError(["kind", "get", "clusters"], result)
        names = {line.strip() for line in (result.stdout or "").splitlines() if line.strip()}
        return self.config.kind_name in names

    def _kind_delete_cluster(self) -> None:
        if not self._kind_cluster_exists():
            return
        self.runner.run(["kind", "delete", "cluster", "--name", self.config.kind_name])

    def _helm_uninstall(self) -> None:
        cmd = ["helm", "uninstall", self.config.release_name, *self._kube_flags(helm=True)]
        result = self.runner.run(cmd, check=False)
        if result.returncode == 0:
            return
        combined = f"{result.stdout} {result.stderr}".lower()
        if "not found" in combined or "release: not found" in combined:
            return
        raise CommandError(cmd, result)

    def _wait_until_ready(self) -> None:
        cmd = [
            "kubectl",
            *self._kube_flags(),
            "wait",
            "--for=condition=available",
            *[f"deployment/{name}" for name in DEPLOYMENTS],
            f"--timeout={self.ready_timeout_seconds}s",
        ]
        try:
            self.runner.run(cmd)
        except CommandError as exc:
            detail = (exc.completed.stderr or exc.completed.stdout or "").strip()
            status = self._deployment_status()
            message = f"Timed out waiting for healthy: {', '.join(DEPLOYMENTS)}"
            if detail:
                message = f"{message}\n{detail}"
            if status:
                message = f"{message}\n{status}"
            raise TimeoutError(message) from exc

    def _deployment_status(self) -> str:
        try:
            result = self.runner.run(
                ["kubectl", *self._kube_flags(), "get", "deploy,pods", "-o", "wide"],
                check=False,
            )
        except CommandError:
            return ""
        return (result.stdout or result.stderr or "").strip()

    def _query_status(self) -> dict[str, WorkloadRow]:
        not_found = {
            name: WorkloadRow(service, name, WorkloadStatus.NOT_FOUND, "not found", "not found")
            for service, name in WORKLOADS
        }
        self.require_tools("kubectl")
        try:
            data = self._kubectl_json(["get", "deploy,pods", "-l", "app.kubernetes.io/part-of=foreverbull"])
        except CommandError:
            return not_found

        items = data.get("items") or []
        deploys = [item for item in items if item.get("kind") == "Deployment"]
        pods = [item for item in items if item.get("kind") == "Pod"]
        deploy_by_name = {item.get("metadata", {}).get("name"): item for item in deploys}
        pods_by_app: dict[str, list[dict[str, Any]]] = {}
        for pod in pods:
            app = (pod.get("metadata") or {}).get("labels", {}).get("app.kubernetes.io/name")
            if app:
                pods_by_app.setdefault(app, []).append(pod)

        filled: dict[str, WorkloadRow] = {}
        for service, name in WORKLOADS:
            filled[name] = self._row_from_resources(
                service,
                name,
                deploy_by_name.get(name),
                (pods_by_app.get(name) or [None])[0],
            )
        return filled

    def _row_from_resources(
        self,
        service: str,
        name: str,
        deploy: dict[str, Any] | None,
        pod: dict[str, Any] | None,
    ) -> WorkloadRow:
        image = "not found"
        container_id = "not found"
        status = WorkloadStatus.NOT_FOUND
        if pod:
            metadata = pod.get("metadata") or {}
            container_id = str(metadata.get("name") or "not found")
            pod_status = pod.get("status") or {}
            phase = str(pod_status.get("phase") or "")
            ready = False
            for cs in pod_status.get("containerStatuses") or []:
                image = str(cs.get("image") or image)
                ready = bool(cs.get("ready"))
                raw_id = str(cs.get("containerID") or "")
                if raw_id:
                    container_id = raw_id.split("://", 1)[-1][:12]
                    break
            if image == "not found":
                spec_containers = (pod.get("spec") or {}).get("containers") or []
                if spec_containers:
                    image = str(spec_containers[0].get("image") or "not found")
            status = _status_from_phase(phase, ready)
        elif deploy:
            replicas = int((deploy.get("spec") or {}).get("replicas") or 0)
            status = WorkloadStatus.STOPPED if replicas == 0 else WorkloadStatus.PENDING
            containers = ((deploy.get("spec") or {}).get("template") or {}).get("spec", {}).get("containers") or []
            if containers:
                image = str(containers[0].get("image") or "not found")
        return WorkloadRow(service, name, status, image, container_id)

    def _kubectl_json(self, resource_args: Sequence[str]) -> dict[str, Any]:
        result = self.runner.run(["kubectl", *self._kube_flags(), *resource_args, "-o", "json"])
        stdout = (result.stdout or "").strip()
        if not stdout:
            return {"items": []}
        loaded = json.loads(stdout)
        if isinstance(loaded, dict):
            return loaded
        return {"items": []}


def _status_from_phase(phase: str, ready: bool) -> WorkloadStatus:
    normalized = phase.lower()
    if normalized == "running":
        return WorkloadStatus.RUNNING if ready else WorkloadStatus.PENDING
    if normalized == "pending":
        return WorkloadStatus.PENDING
    if normalized in {"failed", "error"}:
        return WorkloadStatus.FAILED
    if normalized in {"succeeded", "unknown"}:
        return WorkloadStatus.STOPPED
    if not normalized:
        return WorkloadStatus.NOT_FOUND
    return WorkloadStatus.PENDING
