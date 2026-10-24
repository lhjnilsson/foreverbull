import os
import stat

from pathlib import Path

import pytest

from foreverbull.broker.config import save_cluster_config
from foreverbull.broker.credentials import TokenFileError
from foreverbull.broker.credentials import resolve_token
from foreverbull.broker.credentials import save_token_file
from foreverbull.broker.credentials import set_cli_token
from foreverbull.broker.credentials import token_path


@pytest.fixture(autouse=True)
def _clear_cli_token():
    set_cli_token(None)
    yield
    set_cli_token(None)


def test_resolve_order_trims_and_ignores_cluster_config(tmp_path: Path):
    home = tmp_path / "home"
    config = home / ".foreverbull"
    config.mkdir(parents=True)
    (config / "config.yaml").write_text("cluster: local\ntoken: fb_from_config\n")
    path = token_path(home)
    path.write_text("  fb_file \n")
    path.chmod(0o600)

    assert resolve_token(environ={}, home=home) == "fb_file"
    assert resolve_token(environ={"FOREVERBULL_TOKEN": " fb_env "}, home=home) == "fb_env"
    set_cli_token(" fb_cli ")
    assert resolve_token(environ={"FOREVERBULL_TOKEN": "fb_env"}, home=home) == "fb_cli"
    assert "fb_from_config" not in path.read_text()


def test_loose_token_file_refuses_without_the_secret(tmp_path: Path):
    home = tmp_path / "home"
    path = token_path(home)
    path.parent.mkdir(parents=True)
    path.write_text("fb_hidden_secret")
    path.chmod(0o644)
    with pytest.raises(TokenFileError) as raised:
        resolve_token(environ={}, home=home)
    message = str(raised.value)
    assert str(path) in message
    assert "fb_hidden_secret" not in message


def test_save_token_file_is_0600_and_refuses_without_force(tmp_path: Path):
    home = tmp_path / "home"
    path = save_token_file(" fb_first \n", force=False, home=home)
    assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert path.read_text() == "fb_first"
    with pytest.raises(TokenFileError, match="already exists"):
        save_token_file("fb_second", force=False, home=home)
    assert path.read_text() == "fb_first"
    path.chmod(0o644)
    replaced = save_token_file("fb_second", force=True, home=home)
    assert replaced.read_text() == "fb_second"
    assert stat.S_IMODE(replaced.stat().st_mode) == 0o600
    assert list(path.parent.glob(f".{path.name}.*.tmp")) == []


def test_save_cluster_config_drops_token(tmp_path: Path):
    path = tmp_path / "config.yaml"
    path.write_text("cluster: local\ntoken: fb_secret\n")
    save_cluster_config({"cluster": "remote", "token": "fb_other"}, path=path)
    text = path.read_text()
    assert "fb_secret" not in text
    assert "fb_other" not in text
    assert "token:" not in text
    assert os.path.basename(path) == "config.yaml"
