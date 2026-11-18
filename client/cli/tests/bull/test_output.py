import io

from datetime import date

import pytest

from foreverbull_cli.bull.cli import cli
from foreverbull_cli.output import GRAFANA_DASHBOARD_UID
from foreverbull_cli.output import GRAFANA_DASHBOARD_URL
from foreverbull_cli.output import LiveProgress
from foreverbull_cli.output import execution_link
from foreverbull_cli.output import format_date_field
from foreverbull_cli.output import format_optional_float
from foreverbull_cli.output import format_proto_date
from foreverbull_cli.output import grafana_execution_url
from foreverbull_cli.output import progress_text
from rich.console import Console
from typer.testing import CliRunner

from foreverbull.exceptions import ConfigurationError
from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.pb_utils import from_pydate_to_proto_date


def test_format_proto_date():
    proto = from_pydate_to_proto_date(date(2020, 1, 31))
    assert format_proto_date(proto) == "2020-01-31"
    assert format_proto_date(None) == ""
    assert format_proto_date(common_pb.Date()) == ""
    assert format_proto_date(common_pb.Date(year=2020, month=0, day=1)) == ""


def test_format_date_field_and_optional_float():
    execution = execution_pb.Execution(
        id="exec-1",
        start_date=from_pydate_to_proto_date(date(2020, 1, 1)),
    )
    assert format_date_field(execution, "start_date") == "2020-01-01"
    assert format_date_field(execution, "end_date") == ""

    period = execution_pb.Period(alpha=0.123456)
    assert format_optional_float(period, "alpha") == "0.12346"
    assert format_optional_float(period, "beta") == "-"


def test_execution_link_default(monkeypatch):
    monkeypatch.delenv("GRAFANA_URL", raising=False)
    assert execution_link("exec-1") == f"{GRAFANA_DASHBOARD_URL}?var-execution=exec-1"
    assert execution_link("exec-1") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-1"


def test_grafana_execution_url_builder():
    assert GRAFANA_DASHBOARD_UID == "ee9ep4jxtwkcgf"
    assert grafana_execution_url("", "exec-1") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-1"
    assert (
        grafana_execution_url("http://localhost:3000/", "abc-123")
        == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=abc-123"
    )
    assert (
        grafana_execution_url("https://grafana.example.com", "run-9")
        == "https://grafana.example.com/d/ee9ep4jxtwkcgf?var-execution=run-9"
    )
    assert (
        grafana_execution_url("https://example.com/grafana", "e1")
        == "https://example.com/grafana/d/ee9ep4jxtwkcgf?var-execution=e1"
    )


def test_execution_link_uses_grafana_url_env(monkeypatch):
    monkeypatch.setenv("GRAFANA_URL", "https://grafana.example.com/")
    assert execution_link("exec-1") == "https://grafana.example.com/d/ee9ep4jxtwkcgf?var-execution=exec-1"
    monkeypatch.setenv("GRAFANA_URL", "")
    assert execution_link("exec-1") == "http://localhost:3000/d/ee9ep4jxtwkcgf?var-execution=exec-1"


@pytest.mark.parametrize(
    ("raw", "shown", "secret"),
    [
        ("not a url", "not a url", None),
        ("localhost:3000", "localhost:3000", None),
        ("http://", "http://", None),
        ("ftp://grafana.example.com", "ftp://grafana.example.com", None),
        ("   ", "   ", None),
        ("http://user:hunter2@[::1", "http://user:***@[::1", "hunter2"),
        ("http://user:pw@", "http://user:***@", "pw"),
    ],
)
def test_invalid_grafana_url_names_the_variable(raw, shown, secret):
    message = (
        f"GRAFANA_URL must be an http or https URL with a host, but it's set to '{shown}'. "
        "Set it to a URL like https://grafana.example.com, or unset it to use the local Grafana."
    )
    with pytest.raises(ConfigurationError) as raised:
        grafana_execution_url(raw, "exec-2")
    assert str(raised.value) == message
    if secret is not None:
        assert secret not in str(raised.value)


def test_invalid_grafana_url_fails_the_command_without_a_traceback(monkeypatch):
    monkeypatch.setenv("GRAFANA_URL", "not a url")
    result = CliRunner().invoke(cli, ["backtest", "etfs"])
    assert result.exit_code == 1
    assert result.stdout.strip() == (
        "GRAFANA_URL must be an http or https URL with a host, but it's set to 'not a url'. "
        "Set it to a URL like https://grafana.example.com, or unset it to use the local Grafana."
    )
    assert "Traceback" not in result.stdout
    assert "iShares" not in result.stdout


def test_unset_grafana_url_still_runs(monkeypatch):
    monkeypatch.delenv("GRAFANA_URL", raising=False)
    result = CliRunner().invoke(cli, ["backtest", "etfs"])
    assert result.exit_code == 0
    assert "iShares S&P 100" in result.stdout


def test_valid_grafana_url_still_runs(monkeypatch):
    monkeypatch.setenv("GRAFANA_URL", "https://grafana.example.com")
    result = CliRunner().invoke(cli, ["backtest", "etfs"])
    assert result.exit_code == 0
    assert "iShares S&P 100" in result.stdout


_PROGRESS_COPY = "Downloading prices: 1,240 of 10,000 symbols"


def _create_rsp(**steps):
    return backtest_service_pb.CreateBacktestResponse(**steps)


def _rendered(renderable) -> str:
    out = io.StringIO()
    Console(file=out, width=120, color_system=None).print(renderable)
    return out.getvalue()


def test_live_progress_shows_a_running_steps_progress_in_place_of_its_row():
    progress = LiveProgress()
    progress.update_from_message(_create_rsp())
    downloading = common_pb.Step(state=common_pb.State.RUNNING, message="Downloading prices: 0 of 10,000 symbols")
    progress.update_from_message(
        _create_rsp(CREATING=common_pb.Step(state=common_pb.State.COMPLETED), DOWNLOADING=downloading)
    )
    downloading = common_pb.Step(state=common_pb.State.RUNNING, message=_PROGRESS_COPY)
    progress.update_from_message(
        _create_rsp(CREATING=common_pb.Step(state=common_pb.State.COMPLETED), DOWNLOADING=downloading)
    )

    rendered = _rendered(progress.progress)
    assert _PROGRESS_COPY in rendered.splitlines()[1]
    assert "prices: 0 of 10,000" not in rendered, "the row is replaced, not added to"
    assert len(progress.progress.tasks) == 4

    progress.update_from_message(
        _create_rsp(
            CREATING=common_pb.Step(state=common_pb.State.COMPLETED),
            DOWNLOADING=common_pb.Step(state=common_pb.State.COMPLETED),
        )
    )
    rendered = _rendered(progress.progress)
    assert "Downloading prices" not in rendered
    assert "Downloading" in rendered.splitlines()[1]
    assert progress.notices == [], "progress is not a notice"


_WITH_WAIT = "Downloading prices: 1,240 of 10,000 symbols (paused 30 seconds because yahoo is limiting requests)"
_SIX_SECONDS = "Downloading prices: 1,240 of 10,000 symbols (paused 6 seconds because yahoo is limiting requests)"
_AFTER_WAIT = "Downloading prices: 1,241 of 10,000 symbols"
_PLAIN = "Downloading prices: 1,240 of 10,000 symbols"


def _downloading_line(progress: LiveProgress) -> str:
    matches = [
        task.description for task in progress.progress.tasks if str(task.description).startswith("Downloading prices:")
    ]
    assert len(matches) == 1
    return str(matches[0])


def test_rate_limit_wait_is_named_on_the_progress_line_and_then_cleared():
    assert progress_text(_PLAIN, "yahoo", 30) == _WITH_WAIT
    assert progress_text(_AFTER_WAIT, "", 0) == _AFTER_WAIT

    progress = LiveProgress()
    progress.update_from_message(_create_rsp())
    waiting = common_pb.Step(
        state=common_pb.State.RUNNING,
        message=_PLAIN,
        rate_limit_provider="yahoo",
        rate_limit_wait_seconds=30,
    )
    progress.update_from_message(
        _create_rsp(CREATING=common_pb.Step(state=common_pb.State.COMPLETED), DOWNLOADING=waiting)
    )
    assert _downloading_line(progress) == _WITH_WAIT

    continued = common_pb.Step(state=common_pb.State.RUNNING, message=_AFTER_WAIT)
    progress.update_from_message(
        _create_rsp(CREATING=common_pb.Step(state=common_pb.State.COMPLETED), DOWNLOADING=continued)
    )
    assert _downloading_line(progress) == _AFTER_WAIT
    rendered = _rendered(progress.progress)
    assert _AFTER_WAIT in rendered
    assert "limiting requests" not in rendered


def test_rate_limit_wait_of_six_seconds_is_named():
    assert progress_text(_PLAIN, "yahoo", 6) == _SIX_SECONDS


def test_rate_limit_wait_of_zero_is_not_named():
    assert progress_text(_PLAIN, "yahoo", 0) == _PLAIN
    assert progress_text(_PLAIN, "", 0) == _PLAIN

    progress = LiveProgress()
    progress.update_from_message(_create_rsp())
    quiet = common_pb.Step(state=common_pb.State.RUNNING, message=_PLAIN)
    progress.update_from_message(
        _create_rsp(CREATING=common_pb.Step(state=common_pb.State.COMPLETED), DOWNLOADING=quiet)
    )
    assert _downloading_line(progress) == _PLAIN
    rendered = _rendered(progress.progress)
    assert _PLAIN in rendered
    assert "limiting requests" not in rendered


def test_unknown_rate_limit_provider_is_named_as_sent():
    assert progress_text(_PLAIN, "polygon", 30) == (
        "Downloading prices: 1,240 of 10,000 symbols (paused 30 seconds because polygon is limiting requests)"
    )


def test_empty_rate_limit_provider_says_the_provider():
    assert progress_text(_PLAIN, "", 30) == (
        "Downloading prices: 1,240 of 10,000 symbols (paused 30 seconds because the provider is limiting requests)"
    )


# output.py builds `console = Console()`. Rich reads FORCE_COLOR, NO_COLOR, and TTY_COMPATIBLE.
# TTY_COMPATIBLE is checked before FORCE_COLOR. A non-empty NO_COLOR strips the color bytes
# even when FORCE_COLOR made the stream a terminal. An empty value is the same as unset.
_UNSET = object()


def _color_console(
    monkeypatch,
    *,
    force=_UNSET,
    no=_UNSET,
    tty_compatible=_UNSET,
    isatty=False,
    term="xterm-256color",
    colorterm: str | None = "truecolor",
):
    for name in ("FORCE_COLOR", "NO_COLOR", "TTY_COMPATIBLE"):
        monkeypatch.delenv(name, raising=False)
    if force is not _UNSET:
        monkeypatch.setenv("FORCE_COLOR", force)
    if no is not _UNSET:
        monkeypatch.setenv("NO_COLOR", no)
    if tty_compatible is not _UNSET:
        monkeypatch.setenv("TTY_COMPATIBLE", tty_compatible)
    monkeypatch.setenv("TERM", term)
    if colorterm is None:
        monkeypatch.delenv("COLORTERM", raising=False)
    else:
        monkeypatch.setenv("COLORTERM", colorterm)
    stream = io.StringIO()
    stream.isatty = lambda: isatty
    console = Console(file=stream)
    console.print("[red]hi[/red]")
    return console, stream.getvalue()


def test_force_color_unset_on_a_pipe_prints_no_color(monkeypatch):
    console, text = _color_console(monkeypatch)
    assert console.is_terminal is False
    assert console.no_color is False
    assert text == "hi\n"


def test_force_color_set_colors_a_pipe(monkeypatch):
    console, text = _color_console(monkeypatch, force="1")
    assert console.is_terminal is True
    assert console.no_color is False
    assert text == "\x1b[31mhi\x1b[0m\n"


def test_empty_force_color_does_not_color_a_pipe(monkeypatch):
    console, text = _color_console(monkeypatch, force="")
    assert console.is_terminal is False
    assert text == "hi\n"


def test_no_color_strips_color_when_force_color_is_set(monkeypatch):
    console, text = _color_console(monkeypatch, force="1", no="1")
    assert console.is_terminal is True
    assert console.no_color is True
    assert text == "hi\n"


def test_empty_no_color_does_not_strip_forced_color(monkeypatch):
    console, text = _color_console(monkeypatch, force="1", no="")
    assert console.no_color is False
    assert text == "\x1b[31mhi\x1b[0m\n"


def test_tty_compatible_zero_beats_force_color(monkeypatch):
    console, text = _color_console(monkeypatch, force="1", tty_compatible="0", term="dumb", colorterm=None)
    assert console.is_terminal is False
    assert console.is_dumb_terminal is False
    assert text == "hi\n"


def test_force_color_on_a_dumb_terminal_stays_dumb(monkeypatch):
    console, text = _color_console(monkeypatch, force="1", term="dumb", colorterm=None)
    assert console.is_terminal is True
    assert console.is_dumb_terminal is True
    assert text == "hi\n"


def test_a_tty_is_colored_when_force_color_and_no_color_are_unset(monkeypatch):
    console, text = _color_console(monkeypatch, isatty=True)
    assert console.is_terminal is True
    assert text == "\x1b[31mhi\x1b[0m\n"


def test_force_color_zero_still_counts_as_set(monkeypatch):
    # Any non-empty FORCE_COLOR, including "0", makes Rich treat the stream as a terminal.
    console, text = _color_console(monkeypatch, force="0")
    assert console.is_terminal is True
    assert text == "\x1b[31mhi\x1b[0m\n"
