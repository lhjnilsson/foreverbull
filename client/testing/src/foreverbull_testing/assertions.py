"""Capital checks for algorithm tests.

Share counts use the worker formula ``int(portfolio_value * weight / close)``.
A positive target weight is that share count times the close
``order_target_percent`` used on the portfolio day, divided by ``portfolio_value``.
"""

from collections.abc import Callable
from collections.abc import Sequence
from inspect import Parameter
from inspect import signature
from typing import Any

from sqlalchemy import text

from foreverbull.models.finance import ohlc_provider


SHARE_TOLERANCE = 1
MAGIC_LOT = 10


def expected_shares(portfolio_value: float, weight: float, close: float) -> int:
    """Share count from order_target_percent: int(portfolio_value * weight / close)."""
    return int(portfolio_value * weight / close)


def parameter_default(fn: Callable[..., Any] | None, name: str) -> Any:
    """Default for ``name`` on ``fn``.

    Tests that omit ``cash_reserve`` or ``max_positions`` use this instead of
    hardcoding the strategy's current numbers.
    """
    if fn is None:
        raise TypeError(f"pass {name} or a callable whose signature defaults {name}")
    try:
        param = signature(fn).parameters[name]
    except (KeyError, TypeError, ValueError) as exc:
        raise TypeError(f"{getattr(fn, '__name__', fn)} has no parameter {name}") from exc
    if param.default is Parameter.empty:
        raise TypeError(f"{getattr(fn, '__name__', fn)} has no default for {name}")
    return param.default


def portfolio_value(portfolio: Any) -> float:
    pb = getattr(portfolio, "_pb", None)
    if pb is not None and hasattr(pb, "portfolio_value"):
        return float(pb.portfolio_value)
    return float(portfolio.portfolio_value)


def order_close(portfolio: Any, symbol: str) -> float:
    """Close order_target_percent used for ``symbol`` on the portfolio day."""
    closer = getattr(portfolio, "_close_on_bar", None)
    if callable(closer):
        price: Any = closer(symbol)
        return float(price)
    day = portfolio._pb.timestamp.to_datetime().strftime("%Y-%m-%d")
    query = text("SELECT close FROM ohlc WHERE symbol=:symbol AND provider=:provider AND time::DATE=:dt").bindparams(
        symbol=symbol,
        provider=ohlc_provider(),
        dt=day,
    )
    close = portfolio._db.execute(query).scalar()
    if close is None:
        raise AssertionError(f"no close for {symbol} on {day}")
    return float(close)


def target_amounts(portfolio: Any) -> dict[str, int]:
    """Position plus pending order deltas. ``order.amount`` is a delta, not a target."""
    held = {str(position.symbol): int(position.amount) for position in portfolio.positions}
    targets = dict(held)
    for order in portfolio.pending_orders:
        symbol = str(order.symbol)
        targets[symbol] = targets.get(symbol, 0) + int(order.amount)
    return targets


def positive_targets(portfolio: Any) -> dict[str, int]:
    return {symbol: amount for symbol, amount in target_amounts(portfolio).items() if amount > 0}


def positive_target_weight(portfolio: Any) -> float:
    value = portfolio_value(portfolio)
    if value == 0:
        raise AssertionError("portfolio_value is 0")
    total = 0.0
    for symbol, amount in positive_targets(portfolio).items():
        total += amount * order_close(portfolio, symbol) / value
    return total


def long_notional(portfolio: Any) -> float:
    total = 0.0
    for symbol, amount in positive_targets(portfolio).items():
        total += amount * order_close(portfolio, symbol)
    return total


def assert_expected_shares(
    actual: int,
    portfolio_value: float,
    weight: float,
    close: float,
    *,
    tolerance: int = SHARE_TOLERANCE,
    symbol: str | None = None,
) -> None:
    expected = expected_shares(portfolio_value, weight, close)
    name = f"{symbol} " if symbol else ""
    assert abs(int(actual) - expected) <= tolerance, (
        f"{name}shares {actual} differ from int({portfolio_value} * {weight} / {close}) = {expected} "
        f"by more than {tolerance}"
    )


def assert_targets_sized(
    portfolio: Any,
    weight: float,
    symbols: Sequence[str],
    *,
    tolerance: int = SHARE_TOLERANCE,
) -> None:
    """Each named target is int(portfolio_value * weight / close), within ``tolerance`` shares."""
    value = portfolio_value(portfolio)
    targets = target_amounts(portfolio)
    for symbol in symbols:
        close = order_close(portfolio, symbol)
        assert_expected_shares(
            targets.get(symbol, 0),
            value,
            weight,
            close,
            tolerance=tolerance,
            symbol=symbol,
        )


def assert_at_most_max_positions(
    portfolio: Any,
    max_positions: int | None = None,
    *,
    fn: Callable[..., Any] | None = None,
) -> None:
    """At most ``max_positions`` names have a positive target.

    When ``max_positions`` is omitted, use ``fn``'s default for that parameter.
    """
    if max_positions is None:
        max_positions = int(parameter_default(fn, "max_positions"))
    names = list(positive_targets(portfolio))
    assert len(names) <= max_positions, f"{len(names)} positive targets {names}, max_positions is {max_positions}"


def assert_positive_weights_within_reserve(
    portfolio: Any,
    cash_reserve: float | None = None,
    *,
    fn: Callable[..., Any] | None = None,
) -> None:
    """Sum of positive target weights stays within ``1 - cash_reserve``.

    When ``cash_reserve`` is omitted, use ``fn``'s default for that parameter.
    Each weight uses the same close the order used.
    """
    if cash_reserve is None:
        cash_reserve = float(parameter_default(fn, "cash_reserve"))
    limit = 1.0 - float(cash_reserve)
    total = positive_target_weight(portfolio)
    assert total <= limit + 1e-9, f"positive target weights sum to {total:.6f}, above 1 - cash_reserve ({limit:.6f})"


def assert_flat(portfolio: Any, symbols: Sequence[str]) -> None:
    """Named symbols end flat: target 0, including a held name that was sold out."""
    targets = target_amounts(portfolio)
    for symbol in symbols:
        amount = targets.get(symbol, 0)
        assert amount == 0, f"{symbol} target is {amount}, expected 0"


def assert_not_magic_lot(portfolio: Any) -> None:
    """Fail when every buy or every long target is the fixed 10-share lot."""
    buy_sizes = [int(order.amount) for order in portfolio.pending_orders if order.amount > 0]
    long_targets = list(positive_targets(portfolio).values())
    if buy_sizes:
        assert not all(size == MAGIC_LOT for size in buy_sizes), (
            f"buy orders are a magic {MAGIC_LOT}-share lot; size from portfolio_value"
        )
    if long_targets:
        assert not all(size == MAGIC_LOT for size in long_targets), (
            f"long targets are a magic {MAGIC_LOT}-share lot; size from portfolio_value"
        )
