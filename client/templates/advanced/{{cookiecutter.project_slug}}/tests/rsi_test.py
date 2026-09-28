from datetime import datetime
from inspect import signature

import pandas as pd

from capital import assert_deploys_capital
from {{cookiecutter.project_slug}}.allocation import equal_weight_percent
from {{cookiecutter.project_slug}}.rsi import algo
from {{cookiecutter.project_slug}}.rsi import calculate_rsi
from {{cookiecutter.project_slug}}.rsi import place_orders
from {{cookiecutter.project_slug}}.rsi import select_oversold

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position


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

SYMBOLS = list(OVERSOLD)


def test_rsi_function_parameters():
    calc = signature(calculate_rsi)
    assert calc.parameters["timeperiod"].default == 14
    place = signature(place_orders)
    assert place.parameters["oversold"].default == 30.0
    assert place.parameters["max_positions"].default == 10
    assert place.parameters["cash_reserve"].default == 0.05

    by_name = {fn.name: {p.key for p in fn.parameters} for fn in algo.get_definition().functions}
    assert "timeperiod" in by_name["calculate_rsi"]
    assert {"oversold", "max_positions", "cash_reserve"} <= by_name["place_orders"]


def test_select_oversold_ranks_and_caps():
    rsi = pd.Series(OVERSOLD)
    assert select_oversold(rsi, 30.0, 10) == ["VZ", "MCD", "MMM", "TSM"]
    assert select_oversold(rsi, 30.0, 2) == ["VZ", "MCD"]
    assert select_oversold(rsi, 30.0, 0) == []
    assert "MSFT" not in select_oversold(rsi, 30.0, 10)


def test_calculate_rsi(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), positions=[Position(symbol="AAPL", amount=10)])
    asset = asset_manager.get_asset(datetime(2021, 1, 1), datetime(2021, 3, 31), "AAPL")

    calculate_rsi(asset, portfolio)

    rsi = asset.get_metric("rsi")
    assert rsi
    assert rsi > 0


def _assets(asset_manager: AssetManager):
    return asset_manager.get_assets(
        datetime(2021, 1, 1),
        datetime(2021, 3, 31),
        SYMBOLS,
        metrics={"rsi": OVERSOLD},
    )


def test_place_orders_buys_oversold_and_flattens_leavers(
    asset_manager: AssetManager, portfolio_manager: PortfolioManager
):
    assets = _assets(asset_manager)
    portfolio = portfolio_manager.get_portfolio(
        datetime(2021, 3, 31),
        portfolio_value=100_000,
        positions=[Position(symbol="AAPL", amount=10)],
    )
    place_orders(assets, portfolio)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["AAPL"] == -10
    for symbol in ("VZ", "MCD", "MMM", "TSM"):
        assert orders[symbol] > 0
    assert "MSFT" not in orders
    assert "GOOGL" not in orders
    assert_deploys_capital(portfolio, assets, 0.05)


def test_place_orders_rebalances_held_oversold(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    portfolio = portfolio_manager.get_portfolio(
        datetime(2021, 3, 31),
        portfolio_value=100_000,
        positions=[Position(symbol="MMM", amount=10), Position(symbol="AAPL", amount=10)],
    )
    assets = _assets(asset_manager)
    place_orders(assets, portfolio)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["AAPL"] == -10
    assert "MMM" in orders
    close = float(assets.stock_data.xs("MMM", level="symbol")["close"].iloc[-1])
    weight = equal_weight_percent(4, 0.05, 10)
    expected = int(100_000 * weight / close) - 10
    assert orders["MMM"] == expected
    assert expected != 0
    assert_deploys_capital(portfolio, assets, 0.05)


def test_place_orders_concentrates_when_few_qualify(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    portfolio = portfolio_manager.get_portfolio(datetime(2021, 3, 31), portfolio_value=100_000)
    assets = _assets(asset_manager)
    place_orders(assets, portfolio, oversold=30.0, max_positions=10, cash_reserve=0.05)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    buys = {symbol: amount for symbol, amount in orders.items() if amount > 0}
    assert set(buys) == {"VZ", "MCD", "MMM", "TSM"}
    weight = equal_weight_percent(4, 0.05, 10)
    assert weight == 0.95 / 4
    assert 4 * weight <= 0.95 + 1e-12
    close = float(assets.stock_data.xs("VZ", level="symbol")["close"].iloc[-1])
    assert buys["VZ"] == int(100_000 * weight / close)
    capped = int(100_000 * equal_weight_percent(10, 0.05, 10) / close)
    assert buys["VZ"] > capped
    assert_deploys_capital(portfolio, assets, 0.05)


def test_place_orders_respects_max_positions(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    assets = _assets(asset_manager)
    portfolio = portfolio_manager.get_portfolio(
        datetime(2021, 3, 31),
        portfolio_value=100_000,
        positions=[Position(symbol="AAPL", amount=10)],
    )
    place_orders(assets, portfolio, max_positions=2, cash_reserve=0.05)

    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    buys = {symbol for symbol, amount in orders.items() if amount > 0}
    assert buys == {"VZ", "MCD"}
    assert orders["AAPL"] == -10
    assert "MMM" not in orders
    assert "TSM" not in orders
    assert_deploys_capital(portfolio, assets, 0.05)
