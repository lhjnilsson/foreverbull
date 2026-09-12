from datetime import datetime

from {{cookiecutter.project_slug}}.macd import handle_data

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position


def test_handle_data(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    assets = asset_manager.get_assets(datetime(2021, 1, 1), datetime(2021, 3, 31), ["AAPL", "MSFT"])

    handle_data(assets, portfolio)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders
    assert "AAPL" in orders
