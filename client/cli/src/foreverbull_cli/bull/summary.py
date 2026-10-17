from typing import Literal

from rich.markup import escape
from rich.padding import Padding
from rich.table import Table

from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull_cli.output import console
from foreverbull_cli.output import execution_link


# Eight hex characters of a UUID are enough to tell a user's executions apart;
# `fbull backtest execution` refuses a prefix that matches more than one.
SHORT_EXECUTION_ID_LENGTH = 8


def short_execution_id(execution_id: str) -> str:
    return execution_id[:SHORT_EXECUTION_ID_LENGTH]


def format_percent(fraction: float) -> str:
    """Zipline reports returns and drawdown as fractions, so 0.042 prints as +4.2%."""
    return f"{fraction * 100:+.1f}%"


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
    table.add_row("Max drawdown", format_percent(period.max_drawdown), "", "")
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
) -> None:
    console.print(
        f"[green]✔[/green] Backtest finished: {escape(backtest_name)} · {start_iso} → {end_iso} · provider: {provider}"
    )
    console.print()
    if periods:
        console.print(Padding(metrics_table(benchmark, periods[-1]), (0, 0, 0, 2), expand=False))
    else:
        console.print("  No results were stored for this execution.")
    console.print()
    # soft_wrap leaves wrapping to the terminal, so a pasted command or URL has no inserted newline.
    console.print(
        f"  Details: uv run fbull backtest execution {short_execution_id(execution_id)}",
        highlight=False,
        soft_wrap=True,
    )
    console.print(f"  Charts:  {execution_link(execution_id)}", highlight=False, soft_wrap=True)
