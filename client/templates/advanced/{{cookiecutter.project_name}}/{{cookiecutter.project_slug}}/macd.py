import talib

from foreverbull import Algorithm
from foreverbull import Assets
from foreverbull import Function
from foreverbull import Portfolio


def calculate_macd(df, fastperiod: int = 12, slowperiod: int = 26, signalperiod: int = 9):
    macd, signal, hist = talib.MACD(  # type: ignore
        df["close"],
        fastperiod=fastperiod,
        slowperiod=slowperiod,
        signalperiod=signalperiod,
    )
    out = df.copy()
    out["macd"] = macd
    out["signal"] = signal
    out["hist"] = hist
    return out


def select_positive_macd(latest_macd, max_positions: int) -> list[str]:
    ranked = latest_macd.dropna().sort_values(ascending=False)
    return [str(symbol) for symbol, value in ranked.items() if value > 0][:max_positions]


def handle_data(
    assets: Assets,
    portfolio: Portfolio,
    fastperiod: int = 12,
    slowperiod: int = 26,
    signalperiod: int = 9,
    max_positions: int = 10,
    cash_reserve: float = 0.05,
):
    df = assets.stock_data
    df = df.groupby(level="symbol", group_keys=False).apply(
        calculate_macd,
        fastperiod=fastperiod,
        slowperiod=slowperiod,
        signalperiod=signalperiod,
    )

    latest_macd = df.groupby(level="symbol", group_keys=False).apply(lambda x: x.iloc[-1]["macd"])
    selected = select_positive_macd(latest_macd, max_positions)
    weight = portfolio.equal_weight_percent(len(selected), cash_reserve, max_positions)
    targets = {symbol: weight for symbol in selected}

    for symbol, target in targets.items():
        portfolio.order_target_percent(symbol, target)

    for position in list(portfolio.positions):
        if position.symbol not in targets:
            portfolio.order_target(position.symbol, 0)


algo = Algorithm(functions=[Function(callable=handle_data)])
