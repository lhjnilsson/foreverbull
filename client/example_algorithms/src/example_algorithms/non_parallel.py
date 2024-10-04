import logging

from foreverbull import Algorithm, Assets, Function, Order, Portfolio

logger = logging.getLogger("non_parallel")


def handle_data(assets: Assets, portfolio: Portfolio) -> list[Order] | None:
    orders = []
    for asset in assets:
        logger.debug(f"Handling data for {asset.symbol}")
        stock_data = asset.stock_data
        position = [p for p in portfolio.positions if p.symbol == asset.symbol]
        if len(stock_data) < 30:
            logger.debug(f"Insufficient data for {asset.symbol}")
            return None
        short_mean = stock_data["close"].tail(10).mean()
        long_mean = stock_data["close"].tail(30).mean()
        if short_mean > long_mean and not position:
            logger.debug(f"Buying {asset.symbol}")
            orders.append(Order(symbol=asset.symbol, amount=10))
        elif short_mean < long_mean and position:
            logger.debug(f"Selling {asset.symbol}")
            orders.append(Order(symbol=asset.symbol, amount=-position[0].amount))
        else:
            logger.debug(f"No action for {asset.symbol}")
    return orders


Algorithm(functions=[Function(callable=handle_data)])
