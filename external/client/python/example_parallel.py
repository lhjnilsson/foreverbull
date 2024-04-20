from random import choice

from foreverbull import Algorithm, Asset, Function, Order, Portfolio


def handle_data(
    asset: Asset,
    portfolio: Portfolio,
) -> Order:
    return choice(
        [
            Order(
                symbol=asset.symbol,
                amount=10,
            ),
            Order(
                symbol=asset.symbol,
                amount=-10,
            ),
            None,
        ]
    )


Algorithm(functions=[Function(callable=handle_data)])
