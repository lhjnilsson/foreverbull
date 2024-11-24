import logging

import talib

from foreverbull import Algorithm
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Order
from foreverbull import Portfolio


logger = logging.getLogger(__file__)


def calculate_macd(df):
    macd, signal, hist = talib.MACD(df["Close"], fastperiod=12, slowperiod=26, signalperiod=9)  # type: ignore
    df["macd"] = macd
    df["signal"] = signal
    df["hist"] = hist
    return df


def calculate_volatility(df):
    returns = df["Close"].pct_change()
    volatility = returns.std()
    return volatility


def handle_data(assets: Assets, portfolio: Portfolio) -> list[Order]:
    # orders: list[Order] = []
    df = assets.stock_data

    # Calculate MACD
    df = df.groupby(level="Symbol", group_keys=False).apply(calculate_macd)

    # Get top 10 by macd
    latest_macd = df.groupby(level="Symbol", group_keys=False).apply(lambda x: x.iloc[-1]["macd"])
    sorted_macd = latest_macd.sort_values(ascending=False)  # type: ignore

    for symbol, macd in sorted_macd.head(10).items():
        if macd > 0:
            portfolio.order_target_percent(symbol, 0.1)
    # Calculate size of position to take
    return sorted_macd


algo = Algorithm(functions=[Function(callable=handle_data)])
