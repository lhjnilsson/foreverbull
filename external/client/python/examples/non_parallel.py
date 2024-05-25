import logging

from foreverbull import Algorithm, Assets, Function, Order, Portfolio

logger = logging.getLogger("order_logger")
logger.setLevel(logging.DEBUG)
# create file handler which logs even debug messages
fh = logging.FileHandler("non_parallel.log")
fh.setLevel(logging.DEBUG)
logger.addHandler(fh)


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
            logger.debug(f"Buying {asset.symbol}")
            orders.append(Order(symbol=asset.symbol, amount=10))
        elif short_mean < long_mean and position is not None:
            logger.debug(f"Selling {asset.symbol}")
            orders.append(Order(symbol=asset.symbol, amount=-position.amount))
    return orders


Algorithm(functions=[Function(callable=handle_data)])
