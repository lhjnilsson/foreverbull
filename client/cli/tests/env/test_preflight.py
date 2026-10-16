import socket

from pathlib import Path

import pytest

from foreverbull_cli.env.preflight import DOCKER_MIN_MEMORY_BYTES
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


REPO_KIND_CONFIG = Path(__file__).resolve().parents[4] / "deploy" / "kind" / "cluster.yaml"


class TestCheckTool:
    def test_missing_tool_fails_with_install_url(self):
        check = check_tool("kind", None, "", install_url="https://kind.example/install")
        assert check.state == CheckState.FAIL
        assert check.detail == "kind is not installed."
        assert check.hint == "Install kind: https://kind.example/install"

    def test_installed_tool_passes_with_first_version_line(self):
        check = check_tool("kubectl", "/usr/bin/kubectl", "\nClient Version: v1.31.0\nKustomize Version: v5\n")
        assert check.state == CheckState.PASS
        assert check.detail == "Client Version: v1.31.0"

    def test_installed_tool_without_version_output_shows_path(self):
        check = check_tool("helm", "/usr/bin/helm", "")
        assert check.state == CheckState.PASS
        assert check.detail == "/usr/bin/helm"


class TestDocker:
    def test_parse_running_daemon(self):
        info = parse_docker_info(0, "27.3.1 8589934592\n", "")
        assert info == DockerInfo(running=True, server_version="27.3.1", memory_bytes=8589934592)

    def test_parse_stopped_daemon_keeps_first_error_line(self):
        info = parse_docker_info(1, "", "Cannot connect to the Docker daemon at unix:///var/run/docker.sock.\nmore")
        assert not info.running
        assert info.error == "Cannot connect to the Docker daemon at unix:///var/run/docker.sock."

    def test_parse_unexpected_output_is_not_running(self):
        info = parse_docker_info(0, "27.3.1 lots", "")
        assert not info.running
        assert "unexpected docker info output" in info.error

    def test_daemon_check_fails_with_start_hint(self):
        check = check_docker_daemon(DockerInfo(running=False, error="Cannot connect"))
        assert check.state == CheckState.FAIL
        assert check.detail == "Docker is not running. Cannot connect"
        assert "Start Docker Desktop" in check.hint

    def test_daemon_check_passes(self):
        check = check_docker_daemon(DockerInfo(running=True, server_version="27.3.1", memory_bytes=1))
        assert check.state == CheckState.PASS
        assert "27.3.1" in check.detail

    @pytest.mark.parametrize(
        "memory_bytes,state",
        [
            (2 * 2**30, CheckState.FAIL),
            (DOCKER_MIN_MEMORY_BYTES - 1, CheckState.FAIL),
            (DOCKER_MIN_MEMORY_BYTES, CheckState.PASS),
            # Docker Desktop set to 4 GB reports about 3.8 GiB.
            (int(3.84 * 2**30), CheckState.PASS),
        ],
    )
    def test_memory_floor(self, memory_bytes: int, state: CheckState):
        check = check_docker_memory(DockerInfo(running=True, server_version="27", memory_bytes=memory_bytes))
        assert check.state == state

    def test_memory_failure_names_the_amount_and_the_fix(self):
        check = check_docker_memory(DockerInfo(running=True, server_version="27", memory_bytes=2 * 2**30))
        assert check.detail == "Docker has 2 GB of memory; Foreverbull needs at least 4 GB."
        assert "at least 4 GB" in check.hint

    def test_memory_is_skipped_when_docker_is_down(self):
        check = check_docker_memory(DockerInfo(running=False))
        assert check.state == CheckState.SKIP


class TestHostPorts:
    def test_kind_config_lists_every_published_port(self):
        ports = kind_host_ports(REPO_KIND_CONFIG)
        assert ports[:5] == [5432, 4222, 9000, 50055, 3000]
        assert ports[5:] == list(range(27000, 27016))

    def test_kind_config_without_nodes_has_no_ports(self, tmp_path: Path):
        path = tmp_path / "cluster.yaml"
        path.write_text("kind: Cluster\n")
        assert kind_host_ports(path) == []

    def test_existing_cluster_holds_the_ports_without_probing(self):
        def never(port: int) -> bool:
            raise AssertionError(f"probed {port}")

        check = check_host_ports([5432, 3000], cluster_running=True, is_free=never)
        assert check.state == CheckState.PASS
        assert check.detail == "2 ports held by the foreverbull Kind cluster"

    def test_free_ports_pass(self):
        check = check_host_ports([5432, 3000], cluster_running=False, is_free=lambda port: True)
        assert check.state == CheckState.PASS
        assert check.detail == "2 ports free"

    def test_busy_port_names_the_service_and_how_to_find_the_holder(self):
        check = check_host_ports([5432, 4222, 3000], cluster_running=False, is_free=lambda port: port != 5432)
        assert check.state == CheckState.FAIL
        assert check.detail == "Port 5432 (Postgres) is already in use."
        assert check.hint == (
            "A local Postgres is probably running. Stop it (for example `brew services stop postgresql`) "
            "and run this again."
        )

    def test_other_busy_port_hint_names_the_port_and_lsof(self):
        check = check_host_ports([5432, 50055], cluster_running=False, is_free=lambda port: port != 50055)
        assert check.detail == "Port 50055 (Foreverbull gRPC) is already in use."
        assert check.hint == "Find what is using port 50055 with `lsof -i :50055`, stop it, and run this again."

    def test_several_busy_ports_are_all_listed(self):
        check = check_host_ports([3000, 27003, 5432], cluster_running=False, is_free=lambda port: port == 5432)
        assert check.state == CheckState.FAIL
        assert check.detail == "Ports 3000 (Grafana), 27003 (backtest socket) are already in use."
        assert "`lsof -i :3000`" in check.hint

    def test_probe_error_fails_the_check(self):
        def broken(port: int) -> bool:
            raise PermissionError(13, "Permission denied")

        check = check_host_ports([5432], cluster_running=False, is_free=broken)
        assert check.state == CheckState.FAIL
        assert "Could not probe port 5432" in check.detail

    def test_port_is_free_sees_a_listening_socket(self):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as holder:
            holder.bind(("127.0.0.1", 0))
            holder.listen()
            port = holder.getsockname()[1]
            assert not port_is_free(port)

    def test_port_in_time_wait_is_free(self):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server:
            # docker-proxy and Kind are Go, which sets SO_REUSEADDR on listeners;
            # Linux lets the probe reuse TIME_WAIT only when both sides set it.
            server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            server.bind(("127.0.0.1", 0))
            server.listen()
            port = server.getsockname()[1]
            client = socket.create_connection(("127.0.0.1", port))
            conn, _ = server.accept()
        # The side that closes first keeps the port in TIME_WAIT.
        conn.close()
        client.close()
        assert port_is_free(port)


class TestPreflightError:
    def test_message_names_check_detail_hint_and_no_change(self):
        check = HostCheck("Docker daemon", CheckState.FAIL, "Docker is not running.", "Start Docker.")
        message = str(PreflightError(check))
        assert message.splitlines() == [
            "✖ Docker is not running.",
            "Start Docker.",
            "Nothing was changed.",
        ]

    def test_first_failure_skips_pass_and_skip(self):
        checks = [
            HostCheck("kind", CheckState.PASS, "v0.24"),
            HostCheck("Docker memory", CheckState.SKIP, "Docker is not running."),
            HostCheck("Docker daemon", CheckState.FAIL, "down"),
            HostCheck("Host ports", CheckState.FAIL, "busy"),
        ]
        failed = first_failure(checks)
        assert failed is not None
        assert failed.name == "Docker daemon"
        assert first_failure(checks[:2]) is None
