from collections.abc import Sequence


def deployable_value(portfolio_value: float, cash_reserve: float) -> float:
    return portfolio_value * (1.0 - cash_reserve)


def equal_weight_percent(n_selected: int, cash_reserve: float, max_positions: int) -> float:
    n = min(max_positions, n_selected)
    if n < 1:
        return 0.0
    return (1.0 - cash_reserve) / n


def equal_weight_targets(
    selected: Sequence[str],
    cash_reserve: float,
    max_positions: int,
) -> dict[str, float]:
    names = list(selected)[:max_positions]
    weight = equal_weight_percent(len(names), cash_reserve, max_positions)
    return {symbol: weight for symbol in names}
