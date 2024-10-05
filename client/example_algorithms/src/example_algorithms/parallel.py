import logging

from foreverbull import Algorithm, Asset, Function, Order, Portfolio

logger = logging.getLogger("parallel")


def handle_data(asset: Asset, portfolio: Portfolio) -> Order | None:
    stock_data = asset.stock_data
    position = [p for p in portfolio.positions if p.symbol == asset.symbol]
    if len(stock_data) < 30:
        return None
    short_mean = stock_data["close"].tail(10).mean()
    long_mean = stock_data["close"].tail(30).mean()
    logger.info(f"Symbol {asset.symbol}, short_mean: {short_mean}, long_mean: {long_mean}, date: {asset._as_of}")
    if short_mean > long_mean and not position:
        logger.info(f"Buying {asset.symbol}")
        return Order(symbol=asset.symbol, amount=10)
    elif short_mean < long_mean and position:
        logger.info(f"Selling {asset.symbol}")
        return Order(symbol=asset.symbol, amount=-position[0].amount)
    logger.info(f"Nothing to do for {asset.symbol}")
    return None


algo = Algorithm(functions=[Function(callable=handle_data)])
