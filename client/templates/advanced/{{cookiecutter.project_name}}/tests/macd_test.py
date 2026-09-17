from datetime import datetime
from inspect import signature

import pandas as pd

from {{cookiecutter.project_slug}}.allocation import equal_weight_percent
from {{cookiecutter.project_slug}}.macd import algo
from {{cookiecutter.project_slug}}.macd import calculate_macd
from {{cookiecutter.project_slug}}.macd import handle_data
from {{cookiecutter.project_slug}}.macd import select_positive_macd

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position


def test_macd_function_parameters():
    params = signature(handle_data).parameters
    assert params["fastperiod"].default == 12
    assert params["slowperiod"].default == 26
    assert params["signalperiod"].default == 9
    assert params["max_positions"].default == 10
    assert params["cash_reserve"].default == 0.05

    fn = algo.get_definition().functions[0]
    assert fn.name == "handle_data"
    assert {p.key for p in fn.parameters} >= {
        "fastperiod",
        "slowperiod",
        "signalperiod",
        "max_positions",
        "cash_reserve",
    }


def test_select_positive_macd_ranks_and_caps():
    latest = pd.Series({"AAPL": 1.2, "MSFT": -0.4, "GOOGL": 0.8, "VZ": 0.0})
    assert select_positive_macd(latest, 10) == ["AAPL", "GOOGL"]
    assert select_positive_macd(latest, 1) == ["AAPL"]
    assert select_positive_macd(latest, 0) == []


def _selected(assets, max_positions: int = 10) -> list[str]:
    df = assets.stock_data.groupby(level="symbol", group_keys=False).apply(calculate_macd)
    latest = df.groupby(level="symbol", group_keys=False).apply(lambda x: x.iloc[-1]["macd"])
    return select_positive_macd(latest, max_positions)


def test_handle_data_rebalances_held_and_flattens_leavers(
    asset_manager: AssetManager, portfolio_manager: PortfolioManager
):
    portfolio = portfolio_manager.get_portfolio(
        datetime(2021, 3, 31),
        portfolio_value=100_000,
        positions=[Position(symbol="AAPL", amount=10)],
    )
    assets = asset_manager.get_assets(datetime(2021, 1, 1), datetime(2021, 3, 31), ["AAPL", "MSFT"])
    selected = _selected(assets)

    handle_data(assets, portfolio)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders
    assert "AAPL" in orders
    assert orders["AAPL"] != 10

    weight = equal_weight_percent(len(selected), 0.05, 10)
    if selected:
        assert len(selected) * weight <= 0.95 + 1e-12
    for symbol in selected:
        close = float(assets.stock_data.xs(symbol, level="symbol")["close"].iloc[-1])
        held = 10 if symbol == "AAPL" else 0
        assert orders[symbol] == int(100_000 * weight / close) - held
    if "AAPL" not in selected:
        assert orders["AAPL"] == -10


def test_handle_data_flattens_when_max_positions_zero(
    asset_manager: AssetManager, portfolio_manager: PortfolioManager
):
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    assets = asset_manager.get_assets(datetime(2021, 1, 1), datetime(2021, 3, 31), ["AAPL", "MSFT"])

    handle_data(assets, portfolio, max_positions=0, cash_reserve=0.05)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["AAPL"] == -10
    assert all(amount <= 0 for amount in orders.values())


def test_handle_data_respects_max_positions(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), portfolio_value=100_000)
    assets = asset_manager.get_assets(datetime(2021, 1, 1), datetime(2021, 3, 31), ["AAPL", "MSFT"])
    selected = _selected(assets, max_positions=1)

    handle_data(assets, portfolio, max_positions=1, cash_reserve=0.10)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    buys = [symbol for symbol, amount in orders.items() if amount > 0]
    assert buys == selected
    assert len(buys) <= 1
    if buys:
        close = float(assets.stock_data.xs(buys[0], level="symbol")["close"].iloc[-1])
        assert equal_weight_percent(1, 0.10, 1) == 0.90
        assert orders[buys[0]] == int(100_000 * 0.90 / close)
