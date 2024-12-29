from datetime import datetime

from example_algorithms.momentum.macd import handle_data
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_testing.data import AssetManager
from foreverbull_testing.data import PortfolioManager
from foreverbull_testing.data import Position


def test_handle_data(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    # AAPL has negative MACD, MSFT has positive MACD

    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    assets = asset_manager.get_assets(datetime(2021, 1, 1), datetime(2021, 3, 31), ["AAPL", "MSFT"])

    handle_data(assets, portfolio)

    assert len(portfolio.pending_orders) == 2
    assert finance_pb2.Order(symbol="AAPL", amount=-10) in portfolio.pending_orders
    assert finance_pb2.Order(symbol="MSFT", amount=44) in portfolio.pending_orders
