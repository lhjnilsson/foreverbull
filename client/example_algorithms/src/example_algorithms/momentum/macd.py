import logging

import talib

from foreverbull import Algorithm
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


logger = logging.getLogger(__file__)


def calculate_macd(df):
    macd, signal, hist = talib.MACD(df["close"], fastperiod=12, slowperiod=26, signalperiod=9)  # type: ignore
    df["macd"] = macd
    df["signal"] = signal
    df["hist"] = hist
    return df


def calculate_volatility(df):
    returns = df["close"].pct_change()
    volatility = returns.std()
    return volatility


def handle_data(assets: Assets, portfolio: Portfolio):
    # orders: list[Order] = []
    df = assets.stock_data
    # Calculate MACD
    df = df.groupby(level="symbol", group_keys=False).apply(calculate_macd)

    # Get top 10 by macd
    latest_macd = df.groupby(level="symbol", group_keys=False).apply(lambda x: x.iloc[-1]["macd"])
    sorted_macd = latest_macd.sort_values(ascending=False)  # type: ignore

    to_hold = [symbol for symbol, macd in sorted_macd.head(10).items() if macd > 0]
    to_not_hold = [s for s in assets.symbols if s not in to_hold]

    for symbol in to_hold:
        if symbol not in portfolio.positions:
            portfolio.order_target_percent(symbol, 0.1)

    for symbol in to_not_hold:
        if symbol in portfolio.positions:
            portfolio.order_target(symbol, 0)


algo = Algorithm(functions=[Function(callable=handle_data)])
