from foreverbull import Algorithm, Asset, Assets, Function, Order, Portfolio


def measure_assets(asset: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> None:
    asset.short_mean = asset.stock_data["close"].tail(10).mean()
    asset.long_mean = asset.stock_data["close"].tail(30).mean()


def create_orders(assets: Assets, portfolio: Portfolio) -> list[Order]:
    orders = []
    for asset in assets:
        if len(asset.stock_data) < 30:
            return []
        if asset.short_mean > asset.long_mean and portfolio.get_position(asset) is None:
            orders.append(Order(symbol=asset.symbol, amount=10))
        elif asset.short_mean < asset.long_mean and portfolio.get_position(asset) is not None:
            orders.append(Order(symbol=asset.symbol, amount=-10))
    return orders


Algorithm(
    functions=[
        Function(callable=measure_assets),
        Function(callable=create_orders, run_last=True),
    ],
    namespace={"short_mean": dict[str, float], "long_mean": dict[str, float]},
)
