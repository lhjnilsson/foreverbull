from foreverbull import Algorithm, Assets, Function, Order, Portfolio


def handle_data(assets: Assets, portfolio: Portfolio) -> Order:
    orders = []
    for asset in assets:
        stock_data = asset.stock_data
        if len(stock_data) < 30:
            return None
        short_rolling = stock_data["close"].tail(10).mean()
        long_rolling = stock_data["close"].tail(30).mean()
        if short_rolling.iloc[-1] > long_rolling.iloc[-1]:
            orders.append(Order(symbol=asset.symbol, amount=10))
        elif short_rolling.iloc[-1] < long_rolling.iloc[-1]:
            orders.append(Order(symbol=asset.symbol, amount=-10))
    return orders


Algorithm(functions=[Function(callable=handle_data)])
