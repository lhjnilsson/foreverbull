import logging

from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


logger = logging.getLogger("non_parallel")


def get_means(asset: Asset):
    return asset.stock_data["close"].tail(10).mean(), asset.stock_data["close"].tail(30).mean()


def should_buy(asset: Asset, portfolio: Portfolio):
    short_mean = asset.stock_data["close"].tail(10).mean()
    long_mean = asset.stock_data["close"].tail(30).mean()
    position = [p for p in portfolio.positions if p.symbol == asset.symbol]
    return short_mean > long_mean and not position


def handle_data(assets: Assets, portfolio: Portfolio):
    for asset in assets:
        if len(asset.stock_data) < 30:
            print("CONTINUE: ", asset.symbol, len(asset.stock_data))
            continue
        short_mean, long_mean = get_means(asset)
        position = [p for p in portfolio.positions if p.symbol == asset.symbol]
        if short_mean > long_mean and not position:
            logger.debug(f"Buying {asset.symbol}")
            portfolio.order_target(asset.symbol, 10)
        elif short_mean < long_mean and position:
            logger.debug(f"Selling {asset.symbol}")
            portfolio.order_target(asset.symbol, 0)


algo = Algorithm(functions=[Function(callable=handle_data)])
