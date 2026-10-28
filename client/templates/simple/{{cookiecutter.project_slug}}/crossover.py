from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


def get_means(asset: Asset, short_window: int = 10, long_window: int = 30):
    close = asset.stock_data["close"]
    return close.tail(short_window).mean(), close.tail(long_window).mean()


def select_long_crosses(
    assets: Assets,
    short_window: int,
    long_window: int,
    max_positions: int,
) -> list[str]:
    # Cap the book. A larger short-minus-long gap ranks first; ties keep universe order.
    if max_positions < 1:
        return []
    required = max(short_window, long_window)
    scored: list[tuple[float, str]] = []
    for asset in assets:
        close = asset.stock_data["close"]
        if len(close) < required:
            continue
        short_mean, long_mean = get_means(asset, short_window, long_window)
        if short_mean > long_mean:
            scored.append((float(short_mean - long_mean), asset.symbol))
    scored.sort(key=lambda item: item[0], reverse=True)
    return [symbol for _, symbol in scored[:max_positions]]


def handle_data(
    assets: Assets,
    portfolio: Portfolio,
    short_window: int = 10,
    long_window: int = 30,
    cash_reserve: float = 0.05,
    max_positions: int = 10,
):
    selected = select_long_crosses(assets, short_window, long_window, max_positions)
    # n = min(max_positions, len(selected)): fewer crosses concentrate
    # remaining deployable instead of leaving most of the book in cash.
    weight = portfolio.equal_weight_percent(len(selected), cash_reserve, max_positions)
    targets = {symbol: weight for symbol in selected}

    for symbol, target in targets.items():
        portfolio.order_target_percent(symbol, target)

    for position in list(portfolio.positions):
        if position.symbol not in targets:
            portfolio.order_target(position.symbol, 0)


algo = Algorithm(functions=[Function(callable=handle_data)])
