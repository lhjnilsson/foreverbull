import pandas as pd
import talib

from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


def calculate_rsi(asset: Asset, portfolio: Portfolio, timeperiod: int = 14):
    close = asset.stock_data["close"]
    if len(close) <= timeperiod:
        return
    rsi = talib.RSI(close, timeperiod=timeperiod)  # type: ignore
    last = rsi.iat[-1] if hasattr(rsi, "iat") else rsi[-1]
    if last is None or last != last:
        return
    asset.set_metric("rsi", float(last))


def select_oversold(rsi: pd.Series, oversold: float, max_positions: int) -> list[str]:
    qualified = rsi.dropna()
    qualified = qualified[qualified < oversold].sort_values(ascending=True)
    return [str(symbol) for symbol in qualified.index[:max_positions]]


def place_orders(
    assets: Assets,
    portfolio: Portfolio,
    oversold: float = 30.0,
    max_positions: int = 10,
    cash_reserve: float = 0.05,
):
    selected = select_oversold(assets.get_metrics("rsi"), oversold, max_positions)
    # n = min(max_positions, len(selected)): fewer oversold names concentrate
    # remaining deployable instead of leaving most of the book in cash.
    weight = portfolio.equal_weight_percent(len(selected), cash_reserve, max_positions)
    targets = {symbol: weight for symbol in selected}

    for symbol, target in targets.items():
        portfolio.order_target_percent(symbol, target)

    for position in list(portfolio.positions):
        if position.symbol not in targets:
            portfolio.order_target(position.symbol, 0)


algo = Algorithm(
    functions=[Function(callable=calculate_rsi, run_first=True), Function(callable=place_orders, run_last=True)]
)
