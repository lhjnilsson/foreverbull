import shutil
import subprocess

from pathlib import Path

import pytest
import yaml


CHART_RELATIVE = Path("deploy") / "helm" / "foreverbull"
WORKLOADS = ("postgres", "nats", "minio", "zipline", "foreverbull", "grafana")


def _chart_path() -> Path:
    start = Path(__file__).resolve()
    for path in [start, *start.parents]:
        chart = path / CHART_RELATIVE
        if (chart / "Chart.yaml").is_file():
            return chart
    raise FileNotFoundError("Helm chart not found: deploy/helm/foreverbull")


helm = shutil.which("helm")
pytestmark = pytest.mark.skipif(helm is None, reason="helm is not installed")


def _run_helm(*args: str) -> subprocess.CompletedProcess[str]:
    assert helm is not None
    return subprocess.run([helm, *args], check=False, capture_output=True, text=True)


def _load_docs(rendered: str) -> list[dict]:
    docs = []
    for doc in yaml.safe_load_all(rendered):
        if isinstance(doc, dict) and doc.get("kind"):
            docs.append(doc)
    return docs


def _docs_of_kind(docs: list[dict], kind: str) -> list[dict]:
    return [doc for doc in docs if doc.get("kind") == kind]


@pytest.fixture(scope="module")
def chart() -> Path:
    return _chart_path()


def test_helm_lint(chart: Path):
    result = _run_helm("lint", str(chart))
    assert result.returncode == 0, result.stdout + result.stderr
    assert "0 chart(s) failed" in result.stdout or "1 chart(s) linted" in result.stdout


def test_helm_template_has_rbac_and_no_docker_sock(chart: Path):
    result = _run_helm("template", "foreverbull", str(chart), "--namespace", "foreverbull")
    assert result.returncode == 0, result.stdout + result.stderr
    rendered = result.stdout
    assert "/var/run/docker.sock" not in rendered
    assert "docker.sock" not in rendered

    docs = _load_docs(rendered)
    kinds = {doc["kind"] for doc in docs}
    assert "ServiceAccount" in kinds
    assert "Role" in kinds
    assert "RoleBinding" in kinds

    roles = _docs_of_kind(docs, "Role")
    assert roles
    verbs: set[str] = set()
    resources: set[str] = set()
    for rule in roles[0].get("rules") or []:
        verbs.update(rule.get("verbs") or [])
        resources.update(rule.get("resources") or [])
    assert {"get", "list", "watch", "create", "delete"} <= verbs
    assert "pods" in resources

    bindings = _docs_of_kind(docs, "RoleBinding")
    assert bindings
    subjects = bindings[0].get("subjects") or []
    assert any(s.get("kind") == "ServiceAccount" and s.get("name") == "foreverbull" for s in subjects)

    deployments = {doc["metadata"]["name"]: doc for doc in _docs_of_kind(docs, "Deployment")}
    assert set(WORKLOADS) <= set(deployments)
    broker = deployments["foreverbull"]
    spec = broker["spec"]["template"]["spec"]
    assert spec.get("serviceAccountName") == "foreverbull"
    env = {item["name"]: item.get("value") for item in spec["containers"][0].get("env") or []}
    assert env.get("TRADING_ENABLED") == "false"
    assert env.get("ALPACA_MARKETS_API_KEY") in (None, "")
    assert env.get("ALPACA_MARKETS_API_SECRET") in (None, "")

    grafana_env = {
        item["name"]: item.get("value")
        for item in deployments["grafana"]["spec"]["template"]["spec"]["containers"][0].get("env") or []
    }
    assert grafana_env.get("BROKER_URL") == "foreverbull:50055"


def test_helm_template_hostport_still_has_no_docker_sock(chart: Path):
    result = _run_helm(
        "template",
        "foreverbull",
        str(chart),
        "--namespace",
        "foreverbull",
        "--set",
        "expose.hostPort=true",
        "--set",
        "global.imageTag=local",
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "/var/run/docker.sock" not in result.stdout
    docs = _load_docs(result.stdout)
    broker = next(doc for doc in _docs_of_kind(docs, "Deployment") if doc["metadata"]["name"] == "foreverbull")
    ports = broker["spec"]["template"]["spec"]["containers"][0]["ports"]
    grpc = next(port for port in ports if port.get("name") == "grpc")
    assert grpc.get("hostPort") == 50055
    volumes = broker["spec"]["template"]["spec"].get("volumes") or []
    mounts = broker["spec"]["template"]["spec"]["containers"][0].get("volumeMounts") or []
    assert volumes == []
    assert mounts == []
