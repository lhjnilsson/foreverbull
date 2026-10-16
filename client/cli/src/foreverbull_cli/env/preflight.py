"""Host checks that `fbenv` runs before it changes a cluster.

`fbenv create` and `fbenv start` stop on the first failed check with the
check's fix hint, and stay quiet when every check passes. `fbenv status`
lists every check so the same information is there when debugging.
"""

import errno
import socket

from collections.abc import Callable
from collections.abc import Sequence
from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path

import yaml


# Docker Desktop's "4 GB" setting reports about 3.8 GiB, because the VM kernel
# keeps part of it. The README asks for 4 GB; this floor accepts that setting.
DOCKER_MIN_MEMORY_BYTES = int(3.5 * 2**30)
DOCKER_MEMORY_ADVICE_GB = 4
DOCKER_INFO_TIMEOUT_SECONDS = 15
TOOL_VERSION_TIMEOUT_SECONDS = 10
# Probe both because a process on 127.0.0.1 still blocks Docker's 0.0.0.0 bind.
PORT_PROBE_HOSTS: tuple[str, ...] = ("0.0.0.0", "127.0.0.1")

TOOL_VERSION_COMMANDS: dict[str, tuple[str, ...]] = {
    "kind": ("kind", "version"),
    "helm": ("helm", "version", "--short"),
    "kubectl": ("kubectl", "version", "--client"),
    "docker": ("docker", "version", "--format", "{{.Client.Version}}"),
}

BACKTEST_PORT_FIRST = 27000
BACKTEST_PORT_LAST = 27015
POSTGRES_PORT = 5432
PORT_SERVICES = {
    POSTGRES_PORT: "Postgres",
    4222: "NATS",
    9000: "Minio",
    50055: "Foreverbull gRPC",
    3000: "Grafana",
}


class CheckState(StrEnum):
    PASS = "pass"
    FAIL = "fail"
    SKIP = "skip"


@dataclass(frozen=True)
class HostCheck:
    name: str
    state: CheckState
    detail: str
    hint: str = ""


class PreflightError(RuntimeError):
    def __init__(self, check: HostCheck):
        lines = [f"✖ {check.detail}"]
        if check.hint:
            lines.append(check.hint)
        lines.append("Nothing was changed.")
        super().__init__("\n".join(lines))
        self.check = check


def first_failure(checks: Sequence[HostCheck]) -> HostCheck | None:
    for check in checks:
        if check.state == CheckState.FAIL:
            return check
    return None


def check_tool(name: str, path: str | None, version_output: str, *, install_url: str = "") -> HostCheck:
    if path is None:
        hint = f"Install {name}: {install_url}" if install_url else f"Install {name}."
        return HostCheck(name, CheckState.FAIL, f"{name} is not installed.", hint)
    version = _first_line(version_output)
    return HostCheck(name, CheckState.PASS, version or path)


@dataclass(frozen=True)
class DockerInfo:
    running: bool
    server_version: str = ""
    memory_bytes: int = 0
    error: str = ""


def parse_docker_info(returncode: int, stdout: str, stderr: str) -> DockerInfo:
    """Parse `docker info --format '{{.ServerVersion}} {{.MemTotal}}'`."""
    if returncode != 0:
        return DockerInfo(running=False, error=_first_line(stderr or stdout))
    parts = stdout.split()
    if len(parts) == 2 and parts[1].isdigit():
        return DockerInfo(running=True, server_version=parts[0], memory_bytes=int(parts[1]))
    # With the daemon down, docker 28 can exit 0, print only " 0" on stdout,
    # and put the real reason on stderr.
    reason = _first_line(stderr) or f"unexpected docker info output: {_first_line(stdout)!r}"
    return DockerInfo(running=False, error=reason)


def check_docker_daemon(info: DockerInfo) -> HostCheck:
    if info.running:
        return HostCheck("Docker daemon", CheckState.PASS, f"running (server {info.server_version})")
    detail = "Docker is not running."
    if info.error:
        detail = f"{detail} {info.error}"
    hint = "Start Docker Desktop, or the Docker daemon on Linux (`sudo systemctl start docker`), then run this again."
    return HostCheck("Docker daemon", CheckState.FAIL, detail, hint)


def check_docker_memory(info: DockerInfo) -> HostCheck:
    if not info.running:
        return HostCheck("Docker memory", CheckState.SKIP, "Docker is not running.")
    memory_gb = _gb(info.memory_bytes)
    if info.memory_bytes >= DOCKER_MIN_MEMORY_BYTES:
        return HostCheck("Docker memory", CheckState.PASS, f"{memory_gb} GB available to Docker")
    detail = f"Docker has {memory_gb} GB of memory; Foreverbull needs at least {DOCKER_MEMORY_ADVICE_GB} GB."
    hint = (
        f"Give Docker at least {DOCKER_MEMORY_ADVICE_GB} GB in Docker Desktop's resource settings "
        "(on Linux, free memory on the host), then run this again."
    )
    return HostCheck("Docker memory", CheckState.FAIL, detail, hint)


def kind_host_ports(kind_config: Path) -> list[int]:
    """Host ports the Kind node publishes, in the order the config lists them."""
    loaded = yaml.safe_load(kind_config.read_text()) or {}
    ports: list[int] = []
    for node in loaded.get("nodes") or []:
        for mapping in node.get("extraPortMappings") or []:
            host_port = mapping.get("hostPort")
            if isinstance(host_port, int):
                ports.append(host_port)
    return ports


def port_service(port: int) -> str:
    if BACKTEST_PORT_FIRST <= port <= BACKTEST_PORT_LAST:
        return "backtest socket"
    return PORT_SERVICES.get(port, "Kind port mapping")


def port_is_free(port: int) -> bool:
    for host in PORT_PROBE_HOSTS:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
            # Without this a port in TIME_WAIT after `fbenv stop` reads as busy.
            # A live listener still makes the bind fail with EADDRINUSE.
            probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            try:
                probe.bind((host, port))
            except OSError as exc:
                if exc.errno == errno.EADDRINUSE:
                    return False
                raise
    return True


def check_host_ports(
    ports: Sequence[int],
    *,
    cluster_running: bool,
    is_free: Callable[[int], bool] = port_is_free,
) -> HostCheck:
    if cluster_running:
        return HostCheck("Host ports", CheckState.PASS, f"{len(ports)} ports held by the foreverbull Kind cluster")
    busy: list[int] = []
    for port in ports:
        try:
            free = is_free(port)
        except OSError as exc:
            return HostCheck("Host ports", CheckState.FAIL, f"Could not probe port {port}: {exc}.")
        if not free:
            busy.append(port)
    if not busy:
        return HostCheck("Host ports", CheckState.PASS, f"{len(ports)} ports free")
    named = ", ".join(f"{port} ({port_service(port)})" for port in busy)
    noun = "Port" if len(busy) == 1 else "Ports"
    verb = "is" if len(busy) == 1 else "are"
    first = busy[0]
    if first == POSTGRES_PORT:
        hint = (
            "A local Postgres is probably running. Stop it (for example `brew services stop postgresql`) "
            "and run this again."
        )
    else:
        hint = f"Find what is using port {first} with `lsof -i :{first}`, stop it, and run this again."
    return HostCheck("Host ports", CheckState.FAIL, f"{noun} {named} {verb} already in use.", hint)


def _gb(memory_bytes: int) -> str:
    """Memory as Docker Desktop's settings show it: 2 GB, 3.8 GB."""
    return f"{memory_bytes / 2**30:.1f}".removesuffix(".0")


def _first_line(text: str) -> str:
    for line in (text or "").splitlines():
        stripped = line.strip()
        if stripped:
            return stripped
    return ""
