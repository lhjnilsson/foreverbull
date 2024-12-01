from datetime import datetime

from example_algorithms.momentum.rsi import calculate_rsi
from example_algorithms.momentum.rsi import place_orders
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_testing.data import AssetManager
from foreverbull_testing.data import PortfolioManager
from foreverbull_testing.data import Position


def test_calculate_rsi(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    asset = asset_manager.get_asset(datetime(2021, 1, 1), datetime(2021, 3, 31), "AAPL")

    calculate_rsi(asset, portfolio)

    rsi = asset.get_metric("rsi")
    assert rsi
    assert rsi > 0


def test_place_orders(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    rsi = {
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

    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    assets = asset_manager.get_assets(
        datetime(2021, 1, 1),
        datetime(2021, 3, 31),
        [
            "AAPL",
            "MSFT",
            "GOOGL",
            "MMM",
            "TSLA",
            "AMZN",
            "META",
            "NFLX",
            "NVDA",
            "BABA",
            "TSM",
            "T",
            "VZ",
            "KO",
            "PEP",
            "MCD",
            "WMT",
            "COST",
        ],
        metrics={"rsi": rsi},
    )

    place_orders(assets, portfolio)
    assert len(portfolio.pending_orders) == 5
    assert finance_pb2.Order(symbol="AAPL", amount=-10) in portfolio.pending_orders
    assert finance_pb2.Order(symbol="MCD", amount=48) in portfolio.pending_orders
    assert finance_pb2.Order(symbol="VZ", amount=212) in portfolio.pending_orders
    assert finance_pb2.Order(symbol="TSM", amount=92) in portfolio.pending_orders
    assert finance_pb2.Order(symbol="MMM", amount=71) in portfolio.pending_orders
