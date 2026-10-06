from datetime import UTC
from datetime import datetime

from foreverbull_testing.session import Session

from foreverbull import Algorithm
from foreverbull import Function
from foreverbull.models.finance import Asset
from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio


SYMBOL = "SESS"
AS_OF = datetime(2024, 5, 6, tzinfo=UTC)
CLOSE = 30.0
PORTFOLIO_VALUE = 100_000.0
PERCENT = 0.10


def _bar() -> dict:
    return {
        "symbol": SYMBOL,
        "time": AS_OF,
        "open": CLOSE,
        "high": CLOSE + 1,
        "low": CLOSE - 1,
        "close": CLOSE,
        "volume": 1000,
    }


def test_run_order_target_percent_matches_portfolio_formula(session: Session):
    def handle_data(asset: Asset, portfolio: Portfolio) -> None:
        portfolio.order_target_percent(asset.symbol, PERCENT)

    algorithm = Algorithm(functions=[Function(callable=handle_data)])
    session.bars([_bar()])
    orders = session.run(algorithm, AS_OF, PORTFOLIO_VALUE, [])

    assert len(orders) == 1
    assert orders[0].symbol == SYMBOL
    assert orders[0].amount == int(PORTFOLIO_VALUE * PERCENT / CLOSE)


def test_assets_and_portfolio_are_worker_types(session: Session):
    session.bars([_bar()])
    assets = session.assets([SYMBOL], AS_OF)
    portfolio = session.portfolio(AS_OF, PORTFOLIO_VALUE, [])

    assert type(assets) is Assets
    assert type(portfolio) is Portfolio
    closes = assets.stock_data.xs(SYMBOL, level="symbol")["close"]
    assert float(closes.iloc[-1]) == CLOSE
