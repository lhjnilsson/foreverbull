"""Lines fbull prints from data the broker stored, so a wording change is a swap in one place.

This is the CLI's counterpart of the broker's internal/usercopy: the broker
words its own failures there, and the CLI words here what it builds from
stored fields.
"""

import re

from collections.abc import Sequence
from dataclasses import dataclass

from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.strategy.v1 import strategy_pb


# One block names at most this many later-ticker resolutions, then one line for the rest.
LATER_TICKERS_SHOWN = 5

# The API and the broker log keep this code. The status table shows the sentence.
SKIPPED_MISSED_CLOSE = "skipped_missed_close"
SKIPPED_MISSED_CLOSE_STATUS = "Missed the close. Skipped because of --skip-missed."


def deployment_error_line(deployment_id: str, stored: str, *, many: bool) -> str:
    """One plain line under `fbull strategy status` for a deployment in ERROR.

    many is true when the strategy has more than one ERROR deployment. A
    blank-name sentence then names the deployment id and lowercases its
    leading A. An empty stored error is the report sentence.
    """
    text = stored.strip()
    if text == "":
        return (
            f"⚠ Deployment {deployment_id} stopped with an error, but foreverbull didn't save why. "
            "Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
        )
    if many and text.startswith(("A deployment", "A dry-run deployment")):
        return f"⚠ Deployment {deployment_id}: a{text[1:]}"
    return f"⚠ {text}"


def deployment_error_lines(deployments: Sequence[strategy_pb.Deployment]) -> list[str]:
    """ERROR deployments in the order given, which the broker returns oldest first."""
    failed = [item for item in deployments if item.status == strategy_pb.Deployment.Status.ERROR]
    many = len(failed) > 1
    lines = []
    for item in failed:
        stored = item.error if item.has_field("error") else ""
        lines.append(deployment_error_line(item.id, stored, many=many))
    return lines


def deployment_run_error(stored: str) -> str:
    """The Error cell of `fbull strategy status`. Any other stored text is shown as stored."""
    if stored == SKIPPED_MISSED_CLOSE:
        return SKIPPED_MISSED_CLOSE_STATUS
    return stored


@dataclass(frozen=True)
class LaterTicker:
    """A symbol whose bars create read under a later ticker of the same ISIN.

    symbol is the ticker the backtest asked for and read_as the ticker the
    bars were read and stored under. No day is known: the provider doesn't
    say when the ticker changed.
    """

    symbol: str
    read_as: str


def later_tickers(entity: backtest_pb.Backtest) -> list[LaterTicker]:
    """The backtest's stored resolutions, in the broker's order (by symbol)."""
    return [
        LaterTicker(symbol=resolution.symbol, read_as=resolution.read_as) for resolution in entity.ticker_resolutions
    ]


def later_ticker_line(resolution: LaterTicker) -> str:
    """The line for one symbol read under a later ticker."""
    return f"Prices for {resolution.symbol} come from {resolution.read_as}, the company's later ticker."


def later_tickers_hidden_line(hidden: int) -> str:
    """The line that counts the resolutions past LATER_TICKERS_SHOWN."""
    if hidden == 1:
        return "Plus 1 more symbol whose prices come from a later ticker."
    return f"Plus {hidden} more symbols whose prices come from a later ticker."


# The broker's sentence for a start before the calendar. The last date has to
# be the first session named in the parentheses. The CLI replaces the tail.
_BROKER_START_BEFORE_CALENDAR = re.compile(
    r"This backtest starts on (\d{4}-\d{2}-\d{2}), before the first trading day in "
    r"foreverbull's market calendar \((\d{4}-\d{2}-\d{2})\), so it can't be created\. "
    r"Set the start to \2 or later, then create the backtest again\."
)


def backtest_starts_before_calendar_flag(start: str, first_session: str) -> str:
    """The line when create was given --start."""
    return (
        f"This backtest starts on {start}, before the first trading day in "
        f"foreverbull's market calendar ({first_session}), so it can't be created. "
        f"Run `fbull backtest create` again with --start {first_session} or later."
    )


def backtest_starts_before_calendar_file(start: str, first_session: str) -> str:
    """The line when the start came from a backtest file."""
    return (
        f"This backtest starts on {start}, before the first trading day in "
        f"foreverbull's market calendar ({first_session}), so it can't be created. "
        f"Set start to {first_session} or later in the backtest file, then run "
        f"`fbull backtest create` again."
    )


def backtest_starts_before_calendar(message: str, *, start_flag: bool) -> str:
    """Rewrite the broker's start-before-calendar sentence for this invocation.

    --start and a backtest file name different next steps. Any other message
    is returned unchanged.
    """
    match = _BROKER_START_BEFORE_CALENDAR.fullmatch(message.strip())
    if match is None:
        return message
    start, first_session = match.group(1), match.group(2)
    if start_flag:
        return backtest_starts_before_calendar_flag(start, first_session)
    return backtest_starts_before_calendar_file(start, first_session)


def later_ticker_lines(resolutions: Sequence[LaterTicker]) -> list[str]:
    """The lines `fbull backtest create`, `get`, and the run summary print for later tickers.

    One line per resolution up to LATER_TICKERS_SHOWN, then one line counting
    the rest. No resolution prints nothing. The exact strings are pinned in
    tests/test_usercopy.py.
    """
    lines = [later_ticker_line(resolution) for resolution in resolutions[:LATER_TICKERS_SHOWN]]
    hidden = len(resolutions) - LATER_TICKERS_SHOWN
    if hidden > 0:
        lines.append(later_tickers_hidden_line(hidden))
    return lines
