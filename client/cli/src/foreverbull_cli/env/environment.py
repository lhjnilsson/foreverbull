import copy
import importlib.metadata
import json
import os
import re
import shutil
import subprocess

from collections.abc import Callable
from collections.abc import Mapping
from collections.abc import Sequence
from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
from typing import Any

import yaml

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
from foreverbull_cli.env.preflight import DOCKER_INFO_TIMEOUT_SECONDS
from foreverbull_cli.env.preflight import TOOL_VERSION_COMMANDS
from foreverbull_cli.env.preflight import TOOL_VERSION_TIMEOUT_SECONDS
from foreverbull_cli.env.preflight import CheckState
from foreverbull_cli.env.preflight import DockerInfo
from foreverbull_cli.env.preflight import HostCheck
from foreverbull_cli.env.preflight import PreflightError
from foreverbull_cli.env.preflight import check_docker_daemon
from foreverbull_cli.env.preflight import check_docker_memory
from foreverbull_cli.env.preflight import check_host_ports
from foreverbull_cli.env.preflight import check_tool
from foreverbull_cli.env.preflight import first_failure
from foreverbull_cli.env.preflight import kind_host_ports
from foreverbull_cli.env.preflight import parse_docker_info
from foreverbull_cli.env.preflight import port_is_free


RELEASE_NAME = "foreverbull"
# fbenv runs the broker images that match this package's version, so the
# Python client and the broker always come from the same release.
CLIENT_PACKAGE = "foreverbull"
LOCAL_VERSION = "local"
LOCAL_BUILD_HINT = "Build the images with `task build` from a Foreverbull checkout, then run `VERSION=local {command}`."
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

# kubectl returns at most this many broker log lines, so a crash loop cannot
# flood the terminal.
BROKER_LOG_TAIL_LINES = 20
# Python-only users never set POSTGRES_URL or the port range; Helm does. Each
# known broker error maps to a sentence they can act on, shown before the raw
# log line.
BROKER_ERROR_ADVICE: tuple[tuple[str, str], ...] = (
    (
        "could not reach postgres",
        "The broker can't reach its database. Run `uv run fbenv stop && uv run fbenv start`.",
    ),
    (
        "no free port in the backtest port range",
        (
            "The broker has no free backtest port. Wait for running backtests to finish, "
            "or run `uv run fbenv stop && uv run fbenv start` to free them."
        ),
    ),
)
BROKER_RESTARTED_ADVICE = (
    "The broker stopped with an error. Run `uv run fbenv stop && uv run fbenv start`; "
    "if it stops again, the log lines below say why."
)


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


# Docker's pull errors, matched in order. The first match names the likely cause.
PULL_ERROR_CAUSES: tuple[tuple[str, tuple[str, ...]], ...] = (
    # "no matching manifest for linux/arm64" also contains "manifest for", so platform goes first.
    ("platform", ("no matching manifest for",)),
    (
        "daemon",
        (
            "permission denied while trying to connect to the docker daemon socket",
            "cannot connect to the docker daemon",
        ),
    ),
    ("missing tag", ("manifest unknown", "manifest for")),
    (
        "registry",
        (
            "pull access denied",
            "unauthorized",
            "toomanyrequests",
            "rate limit",
            "service unavailable",
            "bad gateway",
            "internal server error",
        ),
    ),
    (
        "network",
        (
            "dial tcp",
            "no such host",
            "timeout",
            "connection refused",
            "connection reset",
            "network is unreachable",
            "tls handshake",
            "name resolution",
            "unexpected eof",
        ),
    ),
)
PULL_CAUSE_TEXT = {
    "platform": "the image has no build for this machine.",
    "daemon": "fbenv could not talk to Docker.",
    "network": "Docker could not reach the registry.",
    "registry": "the registry refused the download.",
    "missing tag": "that tag does not exist on the registry.",
    "unknown": "Docker could not download it.",
}


def classify_pull_error(output: str) -> str:
    """Return the likely cause of a failed `docker pull`: platform, daemon, missing tag, registry, network, or unknown."""
    lowered = output.lower()
    for cause, needles in PULL_ERROR_CAUSES:
        if any(needle in lowered for needle in needles):
            return cause
    return "unknown"


def pull_error_platform(output: str) -> str:
    """Return the platform Docker named in "no matching manifest for <platform>", or an empty string."""
    match = re.search(r"no matching manifest for (\S+)", output, re.IGNORECASE)
    return match.group(1).rstrip(".,") if match else ""


class ImagePullError(RuntimeError):
    """A `docker pull` failed. The message names the image and the likely cause; `detail` is Docker's output."""

    def __init__(self, image: str, detail: str):
        self.image = image
        self.tag = image.rpartition(":")[2]
        self.detail = detail.strip()
        self.cause = classify_pull_error(self.detail)
        self.platform = pull_error_platform(self.detail)
        super().__init__(f"Couldn't download the Foreverbull image {image}: {PULL_CAUSE_TEXT[self.cause]}")

    def hint(self, command: str) -> str:
        if self.cause == "platform":
            arch = self.platform.split("/")[1] if "/" in self.platform else self.platform
            arch = arch or "unknown architecture"
            return (
                f"This image has no build for your machine ({arch}). "
                f"{LOCAL_BUILD_HINT.format(command=command)} "
                "Or report it at https://github.com/lhjnilsson/foreverbull/issues."
            )
        if self.cause == "daemon":
            return f"Start Docker, or fix your permissions on its socket, then run `{command}` again."
        if self.cause == "missing tag":
            return (
                f"fbenv uses the images for your installed client version ({self.tag}), "
                f"and none are published for it. {LOCAL_BUILD_HINT.format(command=command)}"
            )
        if self.cause == "registry":
            return f"Wait a few minutes and run `{command}` again."
        return f"Check your internet connection and run `{command}` again."


class VersionError(ValueError):
    """VERSION names something fbenv will not run. Only `local` (images built from a checkout) is accepted."""

    def __init__(self, value: str, client_version: str):
        self.value = value
        self.client_version = client_version
        super().__init__(f"VERSION can only be `local`, got `{value}`.")

    def hint(self) -> str:
        return (
            f"fbenv uses the images for your installed client version ({self.client_version}). "
            "Unset VERSION to use them, or set VERSION=local to use images you built with `task build`."
        )


def installed_client_version() -> str:
    """Version of the installed foreverbull Python client. Its images carry the same tag."""
    return importlib.metadata.version(CLIENT_PACKAGE)


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
        stdin: str | None = None,
    ) -> subprocess.CompletedProcess[str]:
        command = [str(a) for a in args]
        try:
            completed = subprocess.run(
                command,
                check=False,
                capture_output=True,
                text=True,
                input=stdin,
                env=None if env is None else {**os.environ, **env},
                timeout=timeout,
            )
        except FileNotFoundError as exc:
            raise MissingToolError(command[0]) from exc
        if check and completed.returncode != 0:
            raise CommandError(command, completed)
        return completed


NO_WORKLOAD = "\u2014"
"""Image and container placeholder for a stopping or stopped service."""


class WorkloadStatus(StrEnum):
    NOT_FOUND = "not_found"
    RUNNING = "running"
    STOPPED = "stopped"
    STOPPING = "stopping"
    PENDING = "pending"
    FAILED = "failed"


@dataclass(frozen=True)
class WorkloadRow:
    service: str
    name: str
    status: str
    image: str
    container_id: str


@dataclass(frozen=True)
class BrokerDiagnosis:
    advice: str
    detail: str


def match_broker_error(logs: str) -> BrokerDiagnosis | None:
    """Return the advice for the last known broker error in logs, with its log line."""
    for line in reversed(logs.splitlines()):
        lowered = line.lower()
        for needle, advice in BROKER_ERROR_ADVICE:
            if needle in lowered:
                return BrokerDiagnosis(advice, line.strip())
    return None


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
        client_version: str | None = None,
    ):
        self._environ = os.environ if environ is None else environ
        self._client_version = client_version
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
    def client_version(self) -> str:
        if self._client_version is None:
            self._client_version = installed_client_version()
        return self._client_version

    @property
    def version(self) -> str:
        """Image tag: the installed client version, or `local` with VERSION=local. Raises VersionError otherwise."""
        requested = str(self._environ.get("VERSION") or "").strip()
        if not requested:
            return self.client_version
        if requested == LOCAL_VERSION:
            return LOCAL_VERSION
        raise VersionError(requested, self.client_version)

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
        port_probe: Callable[[int], bool] = port_is_free,
    ):
        self.config = config
        self.environment = environment
        self.runner = runner or CommandRunner()
        self._repo_root = repo_root
        self.ready_timeout_seconds = ready_timeout_seconds
        self.port_probe = port_probe
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

    def _tools_for_host(self) -> tuple[str, ...]:
        if self.config.is_local:
            return ("kind", "helm", "kubectl", "docker")
        return ("helm", "kubectl")

    def host_checks(self) -> list[HostCheck]:
        """Run every host check. A remote cluster needs only helm and kubectl here."""
        checks = [self._check_tool(name) for name in self._tools_for_host()]
        if not self.config.is_local:
            remote = "Remote cluster; nothing runs on this host."
            checks.append(HostCheck("Docker daemon", CheckState.SKIP, remote))
            checks.append(HostCheck("Docker memory", CheckState.SKIP, remote))
            checks.append(HostCheck("Host ports", CheckState.SKIP, remote))
            return checks
        if not self.runner.which("docker"):
            missing = "docker is not installed."
            checks.append(HostCheck("Docker daemon", CheckState.SKIP, missing))
            checks.append(HostCheck("Docker memory", CheckState.SKIP, missing))
            checks.append(self._check_host_ports(docker_running=False))
            return checks
        info = self._docker_info()
        checks.append(check_docker_daemon(info))
        checks.append(check_docker_memory(info))
        checks.append(self._check_host_ports(docker_running=info.running))
        return checks

    def preflight(self) -> None:
        """Raise PreflightError for the first failed host check. Print nothing when all pass."""
        failed = first_failure(self.host_checks())
        if failed is not None:
            raise PreflightError(failed)

    def _check_tool(self, name: str) -> HostCheck:
        path = self.runner.which(name)
        version_output = ""
        if path is not None:
            version_output = self._quiet_stdout(TOOL_VERSION_COMMANDS[name], timeout=TOOL_VERSION_TIMEOUT_SECONDS)
        return check_tool(name, path, version_output, install_url=TOOL_HINTS.get(name, ""))

    def _quiet_stdout(self, args: Sequence[str], *, timeout: float) -> str:
        """stdout of a command whose failure only means less detail in a check."""
        try:
            result = self.runner.run(args, check=False, timeout=timeout)
        except (MissingToolError, subprocess.TimeoutExpired):
            return ""
        return result.stdout or ""

    def _docker_info(self) -> DockerInfo:
        try:
            result = self.runner.run(
                ["docker", "info", "--format", "{{.ServerVersion}} {{.MemTotal}}"],
                check=False,
                timeout=DOCKER_INFO_TIMEOUT_SECONDS,
            )
        except subprocess.TimeoutExpired:
            return DockerInfo(running=False, error=f"docker info did not answer in {DOCKER_INFO_TIMEOUT_SECONDS} s.")
        return parse_docker_info(result.returncode, result.stdout or "", result.stderr or "")

    def _check_host_ports(self, *, docker_running: bool) -> HostCheck:
        try:
            config_path = self.kind_config_path
        except FileNotFoundError as exc:
            return HostCheck("Host ports", CheckState.FAIL, str(exc), "Run fbenv from a Foreverbull checkout.")
        if not config_path.is_file():
            hint = "Run fbenv from a Foreverbull checkout, or fix kind.config in .foreverbull/config.yaml."
            return HostCheck("Host ports", CheckState.FAIL, f"Kind config not found: {config_path}.", hint)
        try:
            ports = kind_host_ports(config_path)
        except yaml.YAMLError as exc:
            return HostCheck("Host ports", CheckState.FAIL, f"Could not read {config_path}: {exc}.")
        cluster_running = docker_running and self._kind_node_running()
        return check_host_ports(ports, cluster_running=cluster_running, is_free=self.port_probe)

    def _kind_node_running(self) -> bool:
        """True only while the Kind control-plane container runs and so holds the ports."""
        node = f"{self.config.kind_name}-control-plane"
        try:
            result = self.runner.run(
                ["docker", "inspect", "--format", "{{.State.Running}}", node],
                check=False,
                timeout=DOCKER_INFO_TIMEOUT_SECONDS,
            )
        except (MissingToolError, subprocess.TimeoutExpired):
            return False
        return result.returncode == 0 and (result.stdout or "").strip() == "true"

    def broker_diagnosis(self) -> BrokerDiagnosis | None:
        """Explain why the broker is not ready, or return None when it is ready or still starting."""
        status = self._row("foreverbull").status
        if status not in {WorkloadStatus.FAILED, WorkloadStatus.PENDING}:
            return None
        previous_logs = ""
        for previous in (False, True):
            logs = self._broker_logs(previous=previous)
            diagnosis = match_broker_error(logs)
            if diagnosis is not None:
                return diagnosis
            if previous:
                previous_logs = logs
        if previous_logs:
            return BrokerDiagnosis(BROKER_RESTARTED_ADVICE, previous_logs)
        return None

    def _broker_logs(self, *, previous: bool) -> str:
        cmd = ["kubectl", *self._kube_flags(), "logs", "deployment/foreverbull", f"--tail={BROKER_LOG_TAIL_LINES}"]
        if previous:
            cmd.append("--previous")
        try:
            result = self.runner.run(cmd, check=False)
        except MissingToolError:
            return ""
        if result.returncode != 0:
            return ""
        return (result.stdout or "").strip()

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
        self._apply_trading_profiles()
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
        self._apply_trading_profiles()
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

        if missing and self.config.version == LOCAL_VERSION:
            lines = ["Missing local images:"]
            lines.extend(f"  {image}" for image in missing)
            lines.append("Build them first:")
            lines.append("  task build:docker")
            lines.append("  task client:docker:zipline")
            lines.append("  task grafana:build:docker")
            if not docker:
                raise MissingToolError("docker")
            raise RuntimeError("\n".join(lines))

        if missing and docker and self.config.version != LOCAL_VERSION:
            for image in missing:
                try:
                    self.runner.run(["docker", "pull", image])
                except CommandError as exc:
                    completed = exc.completed
                    raise ImagePullError(image, completed.stderr or completed.stdout or str(exc)) from exc
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
            # Local Kind keeps the documented default credentials, but
            # the chart only accepts them with this explicit opt-in;
            # expose.hostPort alone no longer selects them.
            cmd.extend(["--set", "broker.logLevel=debug", "--set", "credentials.allowDefaults=true"])
        if replicas is not None:
            cmd.extend(["--set", f"replicaCount={replicas}"])
        if reuse_values:
            cmd.append("--reuse-values")
        self.runner.run(cmd)
        self._status_cache = None

    def _apply_trading_profiles(self) -> None:
        # Absent file is not an error. kubectl reads the path; do not print the file.
        path = self.environment.path / "trading-profiles.yaml"
        if not path.is_file():
            return
        self.require_tools("kubectl")
        self._ensure_namespace()
        rendered = self.runner.run(
            [
                "kubectl",
                *self._kube_flags(),
                "create",
                "secret",
                "generic",
                "foreverbull-trading",
                f"--from-file=profiles.yaml={path}",
                "--dry-run=client",
                "-o",
                "yaml",
            ]
        )
        self.runner.run(
            ["kubectl", *self._kube_flags(), "apply", "-f", "-"],
            stdin=rendered.stdout,
        )

    def _ensure_namespace(self) -> None:
        cmd = ["kubectl", *self._kube_flags(), "get", "namespace", self.config.namespace]
        result = self.runner.run(cmd, check=False)
        if result.returncode == 0:
            return
        combined = f"{result.stdout} {result.stderr}".lower()
        if "not found" not in combined:
            raise CommandError(cmd, result)
        self.runner.run(["kubectl", *self._kube_flags(), "create", "namespace", self.config.namespace])

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
        if self.config.is_local:
            cmd.extend(["--set", "credentials.allowDefaults=true"])
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
            self._status_cache = None
            diagnosis = self.broker_diagnosis()
            message = f"Timed out waiting for healthy: {', '.join(DEPLOYMENTS)}"
            if diagnosis is not None:
                message = f"{diagnosis.advice}\n{diagnosis.detail}\n{message}"
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
                pods_by_app.get(name) or [],
            )
        return filled

    def _row_from_resources(
        self,
        service: str,
        name: str,
        deploy: dict[str, Any] | None,
        pods: list[dict[str, Any]],
    ) -> WorkloadRow:
        """Build a row from the Deployment's desired replicas and its pods.

        `fbenv stop` scales the Deployment to 0, but its pods keep reporting
        Running and ready until Kubernetes has terminated them. A pod with a
        deletionTimestamp is on its way out, so it never counts as running.
        """
        live = [pod for pod in pods if not (pod.get("metadata") or {}).get("deletionTimestamp")]
        pod = (live or pods or [None])[0]
        image, container_id = _pod_identity(pod) if pod else ("not found", "not found")
        if deploy is not None and image == "not found":
            image = _deploy_image(deploy)

        if deploy is not None and _desired_replicas(deploy) == 0:
            status = WorkloadStatus.STOPPING if pods else WorkloadStatus.STOPPED
        elif live:
            pod_status = live[0].get("status") or {}
            status = _status_from_phase(str(pod_status.get("phase") or ""), _pod_ready(live[0]))
        elif deploy is not None:
            status = WorkloadStatus.PENDING
        elif pods:
            status = WorkloadStatus.STOPPING
        else:
            status = WorkloadStatus.NOT_FOUND
        if status in (WorkloadStatus.STOPPING, WorkloadStatus.STOPPED):
            # Nothing should be running, so a missing image or container is
            # expected rather than broken. Keep "not found" for the latter.
            image, container_id = NO_WORKLOAD, NO_WORKLOAD
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


def _desired_replicas(deploy: dict[str, Any]) -> int:
    return int((deploy.get("spec") or {}).get("replicas") or 0)


def _deploy_image(deploy: dict[str, Any]) -> str:
    containers = ((deploy.get("spec") or {}).get("template") or {}).get("spec", {}).get("containers") or []
    if containers:
        return str(containers[0].get("image") or "not found")
    return "not found"


def _pod_ready(pod: dict[str, Any]) -> bool:
    statuses = (pod.get("status") or {}).get("containerStatuses") or []
    return bool(statuses) and all(bool(cs.get("ready")) for cs in statuses)


def _pod_identity(pod: dict[str, Any]) -> tuple[str, str]:
    """Return the pod's image and short container ID, falling back to its spec and name."""
    image = "not found"
    container_id = str((pod.get("metadata") or {}).get("name") or "not found")
    for cs in (pod.get("status") or {}).get("containerStatuses") or []:
        image = str(cs.get("image") or image)
        raw_id = str(cs.get("containerID") or "")
        if raw_id:
            container_id = raw_id.split("://", 1)[-1][:12]
            break
    if image == "not found":
        spec_containers = (pod.get("spec") or {}).get("containers") or []
        if spec_containers:
            image = str(spec_containers[0].get("image") or "not found")
    return image, container_id


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
