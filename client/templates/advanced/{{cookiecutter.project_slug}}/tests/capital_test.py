from datetime import datetime

import pytest

from capital import assert_deploys_capital
from capital import long_notional
from macd import calculate_macd
from macd import handle_data as macd_handle_data
from macd import select_positive_macd
from rsi import place_orders
from rsi import select_oversold

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager


AS_OF = datetime(2021, 3, 31)
START = datetime(2021, 1, 1)
UNIVERSE = [
    "AAPL",
    "MSFT",
    "GOOGL",
    "AMZN",
    "META",
    "NFLX",
    "NVDA",
    "JPM",
    "JNJ",
    "XOM",
    "WMT",
    "KO",
    "PEP",
    "MCD",
    "VZ",
    "T",
    "MMM",
    "TSM",
]
OVERSOLD = {
    "AAPL": 70,
    "MSFT": 30,
    "GOOGL": 100,
    "MMM": 20,
    "TSLA": 50,
    "AMZN": 80,
    "META": 40,
    "NFLX": 60,
    "NVDA": 90,
    "BABA": 75,
    "TSM": 25,
    "T": 95,
    "VZ": 5,
    "KO": 55,
    "PEP": 85,
    "MCD": 15,
    "WMT": 45,
    "COST": 65,
}


def _rsi_assets(asset_manager: AssetManager):
    return asset_manager.get_assets(START, AS_OF, list(OVERSOLD), metrics={"rsi": OVERSOLD})


def test_rsi_place_orders_deploys_capital(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    cash_reserve = 0.05
    selected = select_oversold(_rsi_assets(asset_manager).get_metrics("rsi"), 30.0, 10)
    assert len(selected) >= 3

    portfolio = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    assets = _rsi_assets(asset_manager)
    place_orders(assets, portfolio, oversold=30.0, max_positions=10, cash_reserve=cash_reserve)

    assert_deploys_capital(portfolio, assets, cash_reserve)


def test_rsi_place_orders_scales_with_portfolio_value(
    asset_manager: AssetManager, portfolio_manager: PortfolioManager
):
    cash_reserve = 0.05
    assets = _rsi_assets(asset_manager)
    small = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    large = portfolio_manager.get_portfolio(AS_OF, portfolio_value=400_000)
    place_orders(assets, small, oversold=30.0, max_positions=10, cash_reserve=cash_reserve)
    place_orders(assets, large, oversold=30.0, max_positions=10, cash_reserve=cash_reserve)

    n_small = long_notional(small, assets)
    n_large = long_notional(large, assets)
    assert_deploys_capital(small, assets, cash_reserve)
    assert_deploys_capital(large, assets, cash_reserve)
    assert n_large / n_small == pytest.approx(4.0, rel=0.10)


def test_rsi_place_orders_concentrated_book_still_deploys_capital(
    asset_manager: AssetManager, portfolio_manager: PortfolioManager
):
    cash_reserve = 0.05
    assets = _rsi_assets(asset_manager)
    portfolio = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    place_orders(assets, portfolio, oversold=30.0, max_positions=10, cash_reserve=cash_reserve)

    buys = {order.symbol for order in portfolio.pending_orders if order.amount > 0}
    assert buys == {"VZ", "MCD", "MMM", "TSM"}
    assert_deploys_capital(portfolio, assets, cash_reserve)


def test_macd_handle_data_deploys_capital(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    cash_reserve = 0.05
    assets = asset_manager.get_assets(START, AS_OF, UNIVERSE)
    portfolio = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    macd_handle_data(assets, portfolio, max_positions=10, cash_reserve=cash_reserve)

    df = assets.stock_data.groupby(level="symbol", group_keys=False).apply(calculate_macd)
    latest = df.groupby(level="symbol", group_keys=False).apply(lambda x: x.iloc[-1]["macd"])
    selected = select_positive_macd(latest, 10)
    assert selected, "expected positive MACD names in this window"
    assert_deploys_capital(portfolio, assets, cash_reserve)


def test_macd_handle_data_scales_with_portfolio_value(
    asset_manager: AssetManager, portfolio_manager: PortfolioManager
):
    cash_reserve = 0.10
    assets = asset_manager.get_assets(START, AS_OF, UNIVERSE)
    small = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    large = portfolio_manager.get_portfolio(AS_OF, portfolio_value=400_000)
    macd_handle_data(assets, small, max_positions=10, cash_reserve=cash_reserve)
    macd_handle_data(assets, large, max_positions=10, cash_reserve=cash_reserve)

    n_small = long_notional(small, assets)
    n_large = long_notional(large, assets)
    assert n_small > 0
    assert_deploys_capital(small, assets, cash_reserve)
    assert_deploys_capital(large, assets, cash_reserve)
    assert n_large / n_small == pytest.approx(4.0, rel=0.10)
