import logging

from foreverbull import Algorithm, Asset, Function, Order, Portfolio

logger = logging.getLogger("parallel")


def handle_data(asset: Asset, portfolio: Portfolio) -> Order:
    logger.debug(f"Handling data for {asset.symbol}")
    stock_data = asset.stock_data
    position = portfolio.get_position(asset)
    logger.debug(f"{asset.symbol} position: {position}")
    if len(stock_data) < 30:
        logger.debug(f"Insufficient data for {asset.symbol}")
        return None
    short_mean = stock_data["close"].tail(10).mean()
    long_mean = stock_data["close"].tail(30).mean()
    if short_mean > long_mean and position is None:
        logger.debug(f"Buying {asset.symbol}, short_mean: {short_mean}, long_mean: {long_mean}")
        return Order(symbol=asset.symbol, amount=10)
    elif short_mean < long_mean and position is not None:
        logger.debug(f"Selling {asset.symbol}, short_mean: {short_mean}, long_mean: {long_mean}")
        return Order(symbol=asset.symbol, amount=-position.amount)
    logger.debug(f"No action for {asset.symbol}")
    return None


Algorithm(functions=[Function(callable=handle_data)])
