from foreverbull import Algorithm, Asset, Function, Order, Portfolio


def handle_data(asset: Asset, portfolio: Portfolio) -> Order:
    stock_data = asset.stock_data
    position = portfolio.get_position(asset)
    if len(stock_data) < 30:
        return None
    short_mean = stock_data["close"].tail(10).mean()
    long_mean = stock_data["close"].tail(30).mean()
    if short_mean > long_mean and position is None:
        return Order(symbol=asset.symbol, amount=10)
    elif short_mean < long_mean and position is not None:
        return Order(symbol=asset.symbol, amount=-position.amount)


Algorithm(functions=[Function(callable=handle_data)])
