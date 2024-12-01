import pandas as pd
import talib

from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


def calculate_rsi(asset: Asset, portfolio: Portfolio):
    rsi = talib.RSI(asset.stock_data["close"], timeperiod=14)  # type: ignore
    asset.set_metric("rsi", rsi.iat[-1])


# Find assets with RSI value under 30
# Buy top 10 stocks with lowest rsi(oversold)
def place_orders(assets: Assets, portfolio: Portfolio):
    rsi = assets.get_metrics("rsi")

    rsi = rsi[rsi < 30]
    assert isinstance(rsi, pd.Series)

    rsi.sort_values(ascending=True, inplace=True)

    to_hold = [str(s) for s in rsi.head(10).keys()]
    to_not_hold = [s for s in assets.symbols if s not in to_hold]

    for symbol in to_hold:
        if symbol not in portfolio.positions:
            portfolio.order_target_percent(symbol, 0.1)

    for symbol in to_not_hold:
        if symbol in portfolio.positions:
            portfolio.order_target(symbol, 0)


algo = Algorithm(
    functions=[Function(callable=calculate_rsi, run_first=True), Function(callable=place_orders, run_last=True)]
)
