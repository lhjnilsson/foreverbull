from typing import Literal

from rich.markup import escape
from rich.padding import Padding
from rich.table import Table
from rich.text import Text

from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull_cli.output import console
from foreverbull_cli.output import execution_link


# Eight hex characters of a UUID are enough to tell a user's executions apart;
# `fbull backtest execution` refuses a prefix that matches more than one.
SHORT_EXECUTION_ID_LENGTH = 8


def empty_universe_notice(etf_symbol: str, start_iso: str, end_iso: str) -> tuple[str, str]:
    """Shown instead of the generic no-trades lines when an ETF universe had no member on any date of the run.

    There is no Sharpe and Sortino line: the universe, not the ratios, is why the run is flat.
    """
    # GetExecution does not return the symbol a backtest was built from, so
    # `fbull backtest run` has none and names the universe instead.
    if etf_symbol:
        what = f"{etf_symbol} had no member stocks"
    else:
        what = "the universe was empty"
    reason = f"No trades: {what} between {start_iso} and {end_iso}, so the strategy had nothing to trade."
    return reason, "Pick another period or universe with `fbull backtest create`, then run it again."


def short_execution_id(execution_id: str) -> str:
    return execution_id[:SHORT_EXECUTION_ID_LENGTH]


def distinguishing_execution_ids(execution_ids: list[str]) -> list[str]:
    """Segment-aligned prefixes long enough to tell the given executions apart.

    Extends past the eight-character short id until the prefix is unique
    among `execution_ids`, then out to the end of that hyphen segment, so
    `01234567-89ab` is shown instead of the indistinguishable `01234567`.
    """
    shown: list[str] = []
    for execution_id in execution_ids:
        length = SHORT_EXECUTION_ID_LENGTH
        while length < len(execution_id) and any(
            other != execution_id and other.startswith(execution_id[:length]) for other in execution_ids
        ):
            length += 1
        segment_end = execution_id.find("-", length)
        if segment_end != -1:
            length = segment_end
        shown.append(execution_id[:length])
    return shown


def format_percent(fraction: float) -> str:
    """Zipline reports returns and drawdown as fractions, so 0.042 prints as +4.2%."""
    return f"{fraction * 100:+.1f}%"


def format_drawdown(fraction: float) -> str:
    """Drawdown is at most zero, so 0 prints as 0.0% rather than +0.0%."""
    percent = fraction * 100
    if round(percent, 1) == 0:
        percent = 0.0
    return f"{percent:.1f}%"


def _ratio(period: execution_pb.Period, field: Literal["sharpe", "sortino"]) -> str:
    if not period.has_field(field):
        return "-"
    return f"{getattr(period, field):.2f}"


def _benchmark_cells(benchmark: str, period: execution_pb.Period) -> tuple[str, str]:
    if not benchmark:
        return "Benchmark", "not set"
    label = f"Benchmark ({escape(benchmark)})"
    if not period.has_field("benchmark_period_return"):
        return label, "not available"
    return label, format_percent(period.benchmark_period_return)


def held_no_position(periods: list[execution_pb.Period]) -> bool:
    """No period closed with a long or a short.

    Zipline fills a daily order on the next bar, so any filled order shows up as a
    held position at some period close. Counts are integers, so no float compare.
    """
    return all(period.longs_count == 0 and period.shorts_count == 0 for period in periods)


def universe_was_empty(response: backtest_service_pb.GetExecutionResponse) -> bool:
    """The server says the ETF universe had no member on any date of the run.

    Both fields are checked: an older broker sends UNSPECIFIED and false, and a
    symbol-list backtest is never reported empty, so both keep the generic notice.
    """
    return response.universe_source == backtest_pb.UniverseSource.ETF and response.universe_empty


def metrics_table(benchmark: str, period: execution_pb.Period) -> Table:
    """The last period carries the run's cumulative return, ratios, and drawdown."""
    table = Table.grid(padding=(0, 2))
    table.add_column(style="bold")
    table.add_column(justify="right")
    table.add_column(style="bold")
    table.add_column(justify="right")
    benchmark_label, benchmark_value = _benchmark_cells(benchmark, period)
    table.add_row("Return", format_percent(period.algorithm_period_return), benchmark_label, benchmark_value)
    table.add_row("Sharpe", _ratio(period, "sharpe"), "Sortino", _ratio(period, "sortino"))
    table.add_row("Max drawdown", format_drawdown(period.max_drawdown), "", "")
    return table


def print_run_summary(
    *,
    backtest_name: str,
    benchmark: str,
    provider: str,
    start_iso: str,
    end_iso: str,
    execution_id: str,
    periods: list[execution_pb.Period],
    universe_empty: bool,
    universe_etf: str = "",
) -> None:
    if not periods:
        console.print("[green]✔[/green] Backtest finished, but no results were stored for this execution.")
        console.print()
    else:
        console.print(
            f"[green]✔[/green] Backtest finished: {escape(backtest_name)} · {start_iso} → {end_iso} · provider: {provider}"
        )
        console.print()
        console.print(Padding(metrics_table(benchmark, periods[-1]), (0, 0, 0, 2), expand=False))
        console.print()
        if held_no_position(periods):
            # Without this line a tradeless run reads as a broken one: zeros and dashes, no reason.
            if universe_empty:
                # Longer than the generic lines, so the padding keeps a wrapped tail under the indent.
                for line in empty_universe_notice(universe_etf, start_iso, end_iso):
                    console.print(Padding(Text(line), (0, 0, 0, 2), expand=False))
            else:
                console.print(
                    f"  No trades: the strategy held no position on any of the {len(periods)} trading days.",
                    highlight=False,
                )
                console.print(
                    "  Return and drawdown stay at 0 and Sharpe and Sortino are not defined.",
                    highlight=False,
                )
            console.print()
    # soft_wrap leaves wrapping to the terminal, so a pasted command or URL has no inserted newline.
    console.print(
        f"  Details: uv run fbull backtest execution {short_execution_id(execution_id)}",
        highlight=False,
        soft_wrap=True,
    )
    console.print(f"  Charts:  {execution_link(execution_id)}", highlight=False, soft_wrap=True)
