from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


def measure_assets(asset: Asset, portfolio: Portfolio, low: int = 10, high: int = 30) -> None:
    short_mean = asset.stock_data["close"].tail(low).mean()
    if type(short_mean) is not float or type(short_mean) is not int:
        return

    long_mean = asset.stock_data["close"].tail(high).mean()
    if type(long_mean) is not float or type(long_mean) is not int:
        return

    asset.set_metric("short_mean", short_mean)
    asset.set_metric("long_mean", long_mean)


def create_orders(assets: Assets, portfolio: Portfolio):
    for asset in assets:
        position = [p for p in portfolio.positions if p.symbol == asset.symbol]
        short_mean = asset.get_metric("short_mean")
        long_mean = asset.get_metric("long_mean")
        if short_mean is None or long_mean is None:
            continue
        if short_mean > long_mean and not position:
            portfolio.order_target(asset.symbol, 10)
        elif short_mean < long_mean and position:
            portfolio.order_target(asset.symbol, 0)


algo = Algorithm(
    functions=[
        Function(callable=measure_assets),
        Function(callable=create_orders, run_last=True),
    ],
    namespaces=["short_mean", "long_mean"],
)
