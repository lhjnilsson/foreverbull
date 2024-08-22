from foreverbull import Algorithm, Asset, Assets, Function, Order, Portfolio


def measure_assets(asset: Asset, portfolio: Portfolio, low: int = 10, high: int = 30) -> None:
    short_mean = asset.stock_data["close"].tail(low).mean()
    if type(short_mean) is not float or type(short_mean) is not int:
        return

    long_mean = asset.stock_data["close"].tail(high).mean()
    if type(long_mean) is not float or type(long_mean) is not int:
        return

    asset.set_metric("short_mean", short_mean)
    asset.set_metric("long_mean", long_mean)


def create_orders(assets: Assets, portfolio: Portfolio) -> list[Order]:
    orders = []
    for asset in assets:
        if len(asset.stock_data) < 30:
            return []
        short_mean = asset.get_metric("short_mean")
        long_mean = asset.get_metric("long_mean")
        if short_mean > long_mean and portfolio.get_position(asset) is None:
            orders.append(Order(symbol=asset.symbol, amount=10))
        elif short_mean < long_mean and portfolio.get_position(asset) is not None:
            orders.append(Order(symbol=asset.symbol, amount=-10))
    return orders


Algorithm(
    functions=[
        Function(callable=measure_assets),
        Function(callable=create_orders, run_last=True),
    ],
    namespaces=["short_mean", "long_mean"],
)
