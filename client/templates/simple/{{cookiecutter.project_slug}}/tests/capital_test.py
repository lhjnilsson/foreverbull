from datetime import datetime

import pytest

from capital import assert_deploys_capital
from capital import long_notional
from crossover import get_means
from crossover import handle_data

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position


AS_OF = datetime(2021, 3, 31)
START = datetime(2021, 1, 1)
UNIVERSE = ["AAPL", "MSFT", "GOOGL", "AMZN", "JPM", "JNJ", "XOM", "WMT", "KO", "PEP"]
CASH_RESERVE = 0.05


def _selected(assets) -> list[str]:
    names = []
    for asset in assets:
        if len(asset.stock_data) < 30:
            continue
        short_mean, long_mean = get_means(asset)
        if short_mean > long_mean:
            names.append(asset.symbol)
    return names


def test_handle_data_deploys_capital(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    assets = asset_manager.get_assets(START, AS_OF, UNIVERSE)
    selected = _selected(assets)
    assert selected, "expected short>long names in this window"

    portfolio = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    handle_data(assets, portfolio)

    assert_deploys_capital(portfolio, assets, CASH_RESERVE)


def test_handle_data_scales_with_portfolio_value(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    assets = asset_manager.get_assets(START, AS_OF, UNIVERSE)
    assert _selected(assets), "expected short>long names in this window"

    small = portfolio_manager.get_portfolio(AS_OF, portfolio_value=100_000)
    large = portfolio_manager.get_portfolio(AS_OF, portfolio_value=400_000)
    handle_data(assets, small)
    handle_data(assets, large)

    n_small = long_notional(small, assets)
    n_large = long_notional(large, assets)
    assert n_small > 0
    assert_deploys_capital(small, assets, CASH_RESERVE)
    assert_deploys_capital(large, assets, CASH_RESERVE)
    assert n_large / n_small == pytest.approx(4.0, rel=0.10)


def test_handle_data_not_magic_lot(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    assets = asset_manager.get_assets(START, AS_OF, UNIVERSE)
    selected = _selected(assets)
    assert selected, "expected short>long names in this window"

    portfolio = portfolio_manager.get_portfolio(
        AS_OF,
        portfolio_value=100_000,
        positions=[Position(symbol="AAPL", amount=10)],
    )
    handle_data(assets, portfolio)

    buys = [order.amount for order in portfolio.pending_orders if order.amount > 0]
    assert buys, "expected buys when names qualify"
    assert_deploys_capital(portfolio, assets, CASH_RESERVE)
