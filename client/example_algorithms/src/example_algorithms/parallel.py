import logging

from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Function
from foreverbull import Portfolio


logger = logging.getLogger("parallel")


def handle_data(asset: Asset, portfolio: Portfolio):
    stock_data = asset.stock_data
    position = [p for p in portfolio.positions if p.symbol == asset.symbol]
    if len(stock_data) < 30:
        return
    short_mean = stock_data["close"].tail(10).mean()
    long_mean = stock_data["close"].tail(30).mean()
    logger.info(f"Symbol {asset.symbol}, short_mean: {short_mean}, long_mean: {long_mean}, date: {asset._as_of}")
    if short_mean > long_mean and not position:
        logger.info(f"Buying {asset.symbol}")
        portfolio.order_target(asset.symbol, 10)
        return
    elif short_mean < long_mean and position:
        logger.info(f"Selling {asset.symbol}")
        portfolio.order_target(asset.symbol, 0)
        return
    logger.info(f"Nothing to do for {asset.symbol}")


algo = Algorithm(functions=[Function(callable=handle_data)])
