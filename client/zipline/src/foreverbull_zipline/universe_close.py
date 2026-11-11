"""Close a held position on the session it leaves the universe.

The fill uses the algorithm's slippage and commission and is stamped on
that session. A name the strategy already flattens is left to that order.
"""

from collections.abc import Callable
from dataclasses import dataclass
from datetime import date
from typing import Literal

from pandas import isnull
from zipline.errors import NoFurtherDataError

from foreverbull_zipline.errors import PlainSentence


_ISSUES = "https://github.com/lhjnilsson/foreverbull/issues"
Reason = Literal["no_price", "rejected"]


@dataclass(frozen=True)
class CloseFailure:
    symbol: str
    session: date
    reason: Reason


def close_failure_line(failures: list[CloseFailure]) -> str:
    """The one line a run stops with when a universe-drop close cannot fill."""
    ordered = sorted(failures, key=lambda item: (item.session, item.symbol))
    reasons = {item.reason for item in ordered}
    if reasons == {"no_price"} and len({item.session for item in ordered}) == 1:
        return _no_price_one_session(ordered)
    if reasons == {"no_price"}:
        return _no_price_sessions(ordered)
    if reasons == {"rejected"} and len(ordered) == 1:
        return _rejected_one(ordered[0])
    if reasons == {"rejected"}:
        return _rejected_many(ordered)
    return _mixed(ordered)


def order_increases(position: int, amount: int) -> bool:
    """True when amount would open a position or add to one, including through zero."""
    if amount == 0:
        return False
    if position == 0:
        return True
    if position > 0:
        return amount > 0 or -amount > position
    return amount < 0 or amount > -position


def close_dropped_positions(algo, data, members: set[str], *, symbol_of: Callable) -> list[str]:
    """Flatten held names that are not in members, at this bar's close.

    Returns the symbols that were closed. Raises PlainSentence naming every
    symbol that could not be closed. A position the strategy's open orders
    already flatten is not closed here.
    """
    failures: list[CloseFailure] = []
    closed: list[str] = []
    session = _session_date(algo)
    for asset, position in list(algo.portfolio.positions.items()):
        amount = int(position.amount)
        if amount == 0:
            continue
        symbol = symbol_of(asset)
        if symbol in members:
            continue
        if _open_amount(algo, asset) == -amount:
            continue
        failure = _close_one(algo, data, asset, symbol, amount, session)
        if failure is None:
            closed.append(symbol)
        else:
            failures.append(failure)
    if failures:
        raise PlainSentence(close_failure_line(failures))
    return closed


def _no_price_one_session(failures: list[CloseFailure]) -> str:
    day = failures[0].session.isoformat()
    names = _join([item.symbol for item in failures])
    return (
        f"foreverbull couldn't close {names} at the close on {day} because there was no close price that session. "
        f"The run was stopped. Check the prices for {names} on {day}, "
        f"or report it at {_ISSUES} with this message."
    )


def _no_price_sessions(failures: list[CloseFailure]) -> str:
    places = _join([f"{item.symbol} at the close on {item.session.isoformat()}" for item in failures])
    checks = _join([f"{item.symbol} on {item.session.isoformat()}" for item in failures])
    return (
        f"foreverbull couldn't close {places} because there was no close price for those sessions. "
        f"The run was stopped. Check the prices for {checks}, "
        f"or report it at {_ISSUES} with this message."
    )


def _rejected_one(failure: CloseFailure) -> str:
    day = failure.session.isoformat()
    return (
        f"foreverbull couldn't close {failure.symbol} at the close on {day} because the close was rejected. "
        "This is a problem in foreverbull, not in your algo. The run was stopped. "
        f"Please report it at {_ISSUES} with this message."
    )


def _rejected_many(failures: list[CloseFailure]) -> str:
    if len({item.session for item in failures}) == 1:
        day = failures[0].session.isoformat()
        names = _join([item.symbol for item in failures])
        where = f"{names} at the close on {day}"
    else:
        where = _join([f"{item.symbol} at the close on {item.session.isoformat()}" for item in failures])
    return (
        f"foreverbull couldn't close {where} because the close was rejected. "
        "This is a problem in foreverbull, not in your algo. The run was stopped. "
        f"Please report it at {_ISSUES} with this message."
    )


def _mixed(failures: list[CloseFailure]) -> str:
    parts: list[str] = []
    for index, item in enumerate(failures):
        because = (
            "because there was no close price that session"
            if item.reason == "no_price"
            else "because the close was rejected"
        )
        place = f"{item.symbol} at the close on {item.session.isoformat()}"
        if index == 0:
            parts.append(f"foreverbull couldn't close {place} {because}")
        else:
            parts.append(f"and it couldn't close {place} {because}")
    return ", ".join(parts) + ". The run was stopped. " + f"Please report it at {_ISSUES} with this message."


def _join(parts: list[str]) -> str:
    if len(parts) <= 1:
        return "".join(parts)
    if len(parts) == 2:
        return f"{parts[0]} and {parts[1]}"
    return ", ".join(parts[:-1]) + " and " + parts[-1]


def _session_date(algo) -> date:
    stamp = algo.get_datetime()
    return date(stamp.year, stamp.month, stamp.day)


def _open_amount(algo, asset) -> int:
    total = 0
    for order in algo.blotter.open_orders.get(asset, ()):
        if order.open:
            total += int(order.amount)
    return total


def _close_one(algo, data, asset, symbol: str, amount: int, session: date) -> CloseFailure | None:
    price = _close_price(data, asset)
    if price is None:
        return CloseFailure(symbol, session, "no_price")
    algo.blotter.cancel_all_orders_for_asset(asset)
    order_id = algo.order(asset, -amount)
    if order_id is None:
        return CloseFailure(symbol, session, "rejected")
    order = algo.blotter.orders[order_id]
    slippage = algo.blotter.slippage_models[type(asset)]
    fills = list(slippage.simulate(data, asset, [order]))
    if len(fills) != 1 or fills[0][1].amount != -amount:
        algo.blotter.cancel(order_id)
        return CloseFailure(symbol, session, "rejected")
    _, txn = fills[0]
    commission = algo.blotter.commission_models[type(asset)].calculate(order, txn)
    order.filled += txn.amount
    order.commission += commission
    order.dt = txn.dt
    algo.metrics_tracker.process_transaction(txn)
    if commission > 0:
        algo.metrics_tracker.process_commission({"asset": order.asset, "order": order, "cost": commission})
    if not order.open:
        algo.blotter.prune_orders([order])
    return None


def _close_price(data, asset):
    try:
        price = data.current(asset, "close")
    except (NoFurtherDataError, KeyError, ValueError):
        return None
    if price is None or isnull(price):
        return None
    return price
