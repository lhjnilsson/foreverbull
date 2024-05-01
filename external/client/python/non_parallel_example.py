from random import choice

from foreverbull import Algorithm, Assets, Function, Order, Portfolio


def handle_data(assets: Assets, portfolio: Portfolio) -> Order:
    orders = []
    for asset in assets:
        order = choice([Order(symbol=asset.symbol, amount=10), Order(symbol=asset.symbol, amount=-10), None])
        if order:
            orders.append(order)
    return orders


Algorithm(functions=[Function(callable=handle_data)])
