import stat

from pathlib import Path

from foreverbull_cli.bull.cli import cli
from typer.testing import CliRunner

from foreverbull.broker.credentials import resolve_token
from foreverbull.broker.credentials import set_cli_token


def test_token_save_is_0600_and_refuses_without_force(tmp_path, monkeypatch):
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.delenv("FOREVERBULL_TOKEN", raising=False)
    set_cli_token(None)
    path = tmp_path / ".foreverbull" / "token"
    project = Path.cwd() / ".foreverbull" / "config.yaml"
    before = project.read_bytes() if project.is_file() else None
    runner = CliRunner()

    created = runner.invoke(cli, ["token", "save"], input=" fb_first \n")
    assert created.exit_code == 0, created.output
    assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert path.read_text() == "fb_first"
    assert "fb_first" not in created.output

    refused = runner.invoke(cli, ["token", "save"], input="fb_second\n")
    assert refused.exit_code == 1
    assert path.read_text() == "fb_first"
    assert "fb_second" not in refused.output
    assert "fb_second" not in str(refused.exception or "")
    # Rich wraps the path. The secret stays out of the message.
    assert str(path) in refused.output.replace("\n", "")

    replaced = runner.invoke(cli, ["token", "save", "--force"], input="fb_second\n")
    assert replaced.exit_code == 0, replaced.output
    assert path.read_text() == "fb_second"
    assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert "fb_second" not in replaced.output

    home_config = tmp_path / ".foreverbull" / "config.yaml"
    assert not home_config.exists()
    after = project.read_bytes() if project.is_file() else None
    assert before == after


def test_token_flag_is_trimmed_and_not_written_to_config(tmp_path, monkeypatch):
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.delenv("FOREVERBULL_TOKEN", raising=False)
    set_cli_token(None)
    result = CliRunner().invoke(cli, ["--token", " fb_flag ", "token", "save"], input="fb_stdin\n")
    assert result.exit_code == 0, result.output

    assert resolve_token() == "fb_flag"
    token_file = tmp_path / ".foreverbull" / "token"
    assert token_file.read_text() == "fb_stdin"
    assert stat.S_IMODE(token_file.stat().st_mode) == 0o600
    assert not (tmp_path / ".foreverbull" / "config.yaml").exists()
    assert "fb_flag" not in result.output
    assert "fb_stdin" not in result.output
