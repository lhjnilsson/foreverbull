from foreverbull import Algorithm, Assets, Function, Order, Portfolio


def handle_data(assets: Assets, portfolio: Portfolio) -> Order:
    orders = []
    for asset in assets:
        stock_data = asset.stock_data
        position = portfolio.get_position(asset)
        if len(stock_data) < 30:
            return None
        short_mean = stock_data["close"].tail(10).mean()
        long_mean = stock_data["close"].tail(30).mean()
        if short_mean > long_mean and position is None:
            orders.append(Order(symbol=asset.symbol, amount=10))
        elif short_mean < long_mean and position is not None:
            orders.append(Order(symbol=asset.symbol, amount=-position.amount))
    return orders


Algorithm(functions=[Function(callable=handle_data)])
