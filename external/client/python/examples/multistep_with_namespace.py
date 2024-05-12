import random

from foreverbull import Algorithm, Asset, Assets, Function, Order, Portfolio


def measure_assets(asset: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> None:
    asset.metric = random.uniform(10, 99.9)


def create_orders(assets: Assets, portfolio: Portfolio) -> list[Order]:
    metrics = sorted(assets.metric, lambda m: m[1])
    return [
        Order(symbol=metrics[0][0], amount=-10),
        Order(symbol=metrics[-1][0], amount=10),
    ]


Algorithm(
    functions=[
        Function(callable=measure_assets),
        Function(callable=create_orders, run_last=True),
    ],
    namespace={"metric": dict[str, float]},
)
