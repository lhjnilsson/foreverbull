import importlib.util
import subprocess

from pathlib import Path

import pytest


def _demo():
    path = Path(__file__).resolve().parents[2] / "tests" / "end_to_end" / "test_demo.py"
    spec = importlib.util.spec_from_file_location("foreverbull_e2e_test_demo", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _failed(returncode: int, stdout: str, stderr: str) -> subprocess.CompletedProcess[str]:
    return subprocess.CompletedProcess(args=["kubectl"], returncode=returncode, stdout=stdout, stderr=stderr)


def test_kubectl_failure_names_the_exit_code_and_stderr(monkeypatch):
    demo = _demo()
    monkeypatch.setattr(
        demo.subprocess,
        "run",
        lambda *args, **kwargs: _failed(3, "", "The connection to the server was refused\n"),
    )

    with pytest.raises(AssertionError) as raised:
        demo.kubectl("get", "pods")

    assert str(raised.value) == ("kubectl exited 3\nstderr:\nThe connection to the server was refused\nstdout:\n")


def test_kubectl_failure_keeps_stdout_beside_stderr(monkeypatch):
    demo = _demo()
    monkeypatch.setattr(demo.subprocess, "run", lambda *args, **kwargs: _failed(1, "partial\n", "denied\n"))

    with pytest.raises(AssertionError) as raised:
        demo.zipline_bundle("state", "demo")

    assert str(raised.value) == "kubectl exited 1\nstderr:\ndenied\nstdout:\npartial"


def test_kubectl_returns_stdout_when_the_command_exits_zero(monkeypatch):
    demo = _demo()
    monkeypatch.setattr(demo.subprocess, "run", lambda *args, **kwargs: _failed(0, "pod-a\n", "warning\n"))

    assert demo.kubectl("get", "pods") == "pod-a\n"
