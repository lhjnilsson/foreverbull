MAGIC_LOT = 10


def last_closes(assets) -> dict[str, float]:
    last = assets.stock_data.groupby(level="symbol")["close"].last()
    return {str(symbol): float(price) for symbol, price in last.items()}


def target_amounts(portfolio) -> dict[str, int]:
    held = {position.symbol: int(position.amount) for position in portfolio.positions}
    targets = dict(held)
    for order in portfolio.pending_orders:
        targets[order.symbol] = held.get(order.symbol, 0) + int(order.amount)
    return targets


def long_notional(portfolio, assets) -> float:
    closes = last_closes(assets)
    total = 0.0
    for symbol, amount in target_amounts(portfolio).items():
        if amount > 0:
            total += amount * closes[symbol]
    return total


def assert_not_magic_lot(portfolio) -> None:
    buy_sizes = [int(order.amount) for order in portfolio.pending_orders if order.amount > 0]
    long_targets = [amount for amount in target_amounts(portfolio).values() if amount > 0]
    if buy_sizes:
        assert not all(size == MAGIC_LOT for size in buy_sizes), (
            f"buy orders are a magic {MAGIC_LOT}-share lot; size from portfolio_value"
        )
    if long_targets:
        assert not all(size == MAGIC_LOT for size in long_targets), (
            f"long targets are a magic {MAGIC_LOT}-share lot; size from portfolio_value"
        )


def assert_deploys_capital(portfolio, assets, cash_reserve: float, min_fraction: float = 0.80) -> None:
    assert_not_magic_lot(portfolio)
    longs = {symbol: amount for symbol, amount in target_amounts(portfolio).items() if amount > 0}
    assert longs, "expected long targets when there are enough signals"
    deployable = portfolio.deployable_value(cash_reserve)
    notional = long_notional(portfolio, assets)
    assert notional <= deployable + 1e-6, f"long notionals {notional:.2f} exceed deployable {deployable:.2f}"
    assert notional >= min_fraction * deployable, (
        f"long notionals {notional:.2f} use too little of deployable {deployable:.2f}"
    )
