from pathlib import Path
from unittest.mock import MagicMock

import pytest

from foreverbull.broker import backtest
from foreverbull.broker import finance
from foreverbull.broker.config import resolve_broker_address
from foreverbull.broker.config import split_broker_address


def test_resolve_default_is_localhost(tmp_path: Path):
    addr = resolve_broker_address(cwd=tmp_path, home=tmp_path / "home", environ={})
    assert addr == "localhost:50055"


def test_resolve_remote_from_project_config(tmp_path: Path):
    cwd = tmp_path / "proj"
    (cwd / ".foreverbull").mkdir(parents=True)
    (cwd / ".foreverbull" / "config.yaml").write_text(
        "cluster: remote\nremote:\n  broker_address: broker.example:50055\n"
    )
    addr = resolve_broker_address(cwd=cwd, home=tmp_path / "home", environ={})
    assert addr == "broker.example:50055"


def test_resolve_env_overrides_config(tmp_path: Path):
    cwd = tmp_path / "proj"
    (cwd / ".foreverbull").mkdir(parents=True)
    (cwd / ".foreverbull" / "config.yaml").write_text("cluster: remote\nremote:\n  broker_address: from-file:1\n")
    addr = resolve_broker_address(
        cwd=cwd,
        home=tmp_path / "home",
        environ={"FOREVERBULL_BROKER_ADDRESS": "from-env:9"},
    )
    assert addr == "from-env:9"


def test_resolve_remote_requires_broker_address(tmp_path: Path):
    cwd = tmp_path / "proj"
    (cwd / ".foreverbull").mkdir(parents=True)
    (cwd / ".foreverbull" / "config.yaml").write_text("cluster: remote\n")
    with pytest.raises(ValueError, match="remote.broker_address"):
        resolve_broker_address(cwd=cwd, home=tmp_path / "home", environ={})


def test_split_broker_address():
    assert split_broker_address("localhost:50055") == ("localhost", 50055)
    assert split_broker_address("broker.example") == ("broker.example", 50055)


def test_backtest_stub_uses_resolved_address(monkeypatch):
    monkeypatch.setenv("FOREVERBULL_BROKER_ADDRESS", "remote.example:1234")
    seen: list[str] = []
    stub = MagicMock()
    stub.list_backtests.return_value.backtests = []
    monkeypatch.setattr("foreverbull.broker.backtest.grpc.insecure_channel", lambda addr: seen.append(addr) or object())
    monkeypatch.setattr(
        "foreverbull.broker.backtest.backtest_service_pb_grpc.BacktestServicerClientSync",
        lambda channel: stub,
    )
    assert backtest.list_backtests() == []
    assert seen == ["remote.example:1234"]


def test_finance_stub_uses_resolved_address(monkeypatch):
    monkeypatch.setenv("FOREVERBULL_BROKER_ADDRESS", "remote.example:1234")
    seen: list[str] = []
    stub = MagicMock()
    stub.get_portfolio.return_value = "portfolio"
    monkeypatch.setattr("foreverbull.broker.finance.grpc.insecure_channel", lambda addr: seen.append(addr) or object())
    monkeypatch.setattr(
        "foreverbull.broker.finance.trading_service_pb_grpc.TradingServicerClientSync",
        lambda channel: stub,
    )
    assert finance.get_portfolio() == "portfolio"
    assert seen == ["remote.example:1234"]


def test_backtest_stub_uses_config_broker_address(tmp_path: Path, monkeypatch):
    cwd = tmp_path / "proj"
    home = tmp_path / "home"
    (cwd / ".foreverbull").mkdir(parents=True)
    home.mkdir()
    (cwd / ".foreverbull" / "config.yaml").write_text(
        "cluster: remote\nremote:\n  broker_address: broker.example:50055\n"
    )
    monkeypatch.chdir(cwd)
    monkeypatch.setenv("HOME", str(home))
    monkeypatch.delenv("FOREVERBULL_BROKER_ADDRESS", raising=False)
    monkeypatch.delenv("FOREVERBULL_CONFIG", raising=False)
    seen: list[str] = []
    stub = MagicMock()
    stub.list_backtests.return_value.backtests = []
    monkeypatch.setattr("foreverbull.broker.backtest.grpc.insecure_channel", lambda addr: seen.append(addr) or object())
    monkeypatch.setattr(
        "foreverbull.broker.backtest.backtest_service_pb_grpc.BacktestServicerClientSync",
        lambda channel: stub,
    )
    assert backtest.list_backtests() == []
    assert seen == ["broker.example:50055"]
