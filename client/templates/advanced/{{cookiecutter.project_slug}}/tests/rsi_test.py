from datetime import datetime
from inspect import signature

import pandas as pd

from allocation import equal_weight_percent
from rsi import algo
from rsi import calculate_rsi
from rsi import place_orders
from rsi import select_oversold

from foreverbull_testing.assertions import assert_at_most_max_positions
from foreverbull_testing.assertions import assert_flat
from foreverbull_testing.assertions import assert_positive_weights_within_reserve
from foreverbull_testing.assertions import assert_targets_sized
from foreverbull_testing.assertions import expected_shares
from foreverbull_testing.assertions import order_close
from foreverbull_testing.assertions import parameter_default
from foreverbull_testing.assertions import positive_targets
from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position
from foreverbull_testing.session import Session


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

# Synthetic book. AAPL stays out of the inserted bars so the indicator test can download it.
METRICS = {symbol: value for symbol, value in OVERSOLD.items() if symbol != "AAPL"}
PRICES = {symbol: 25.0 + index * 10.0 for index, symbol in enumerate(METRICS)}


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


def _book(session: Session):
    times = None
    for symbol, price in PRICES.items():
        times = session.constant(symbol, 5, price)
    assert times is not None
    assets = session.assets(list(PRICES), times[-1])
    assets.set_metrics("rsi", dict(METRICS))
    return assets, times[-1]


def _limits():
    return (
        parameter_default(place_orders, "cash_reserve"),
        parameter_default(place_orders, "max_positions"),
        parameter_default(place_orders, "oversold"),
    )


def test_place_orders_buys_oversold_and_flattens_leavers(session: Session, namespace_server):
    assets, as_of = _book(session)
    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="GOOGL", amount=10)])
    place_orders(assets, portfolio)

    cash_reserve, max_positions, oversold = _limits()
    selected = select_oversold(assets.get_metrics("rsi"), oversold, max_positions)
    assert selected == ["VZ", "MCD", "MMM", "TSM"]
    assert_flat(portfolio, ["GOOGL"])
    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["GOOGL"] == -10
    assert "MSFT" not in orders
    assert_at_most_max_positions(portfolio, fn=place_orders)
    assert_positive_weights_within_reserve(portfolio, fn=place_orders)
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_targets_sized(portfolio, weight, selected)
    assert namespace_server.get("rsi")


def test_place_orders_rebalances_held_oversold(session: Session, namespace_server):
    assets, as_of = _book(session)
    portfolio = session.portfolio(
        as_of,
        100_000,
        positions=[Position(symbol="MMM", amount=10), Position(symbol="GOOGL", amount=10)],
    )
    place_orders(assets, portfolio)

    cash_reserve, max_positions, oversold = _limits()
    selected = select_oversold(assets.get_metrics("rsi"), oversold, max_positions)
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_flat(portfolio, ["GOOGL"])
    assert "MMM" in selected
    assert_targets_sized(portfolio, weight, selected)
    assert_positive_weights_within_reserve(portfolio, fn=place_orders)
    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["GOOGL"] == -10
    close = order_close(portfolio, "MMM")
    expected = expected_shares(100_000, weight, close) - 10
    assert abs(orders["MMM"] - expected) <= 1
    assert expected != 0
    assert namespace_server.get("rsi")


def test_place_orders_concentrates_when_few_qualify(session: Session, namespace_server):
    assets, as_of = _book(session)
    portfolio = session.portfolio(as_of, 100_000)
    place_orders(assets, portfolio, oversold=30.0, max_positions=10, cash_reserve=0.05)

    buys = positive_targets(portfolio)
    assert set(buys) == {"VZ", "MCD", "MMM", "TSM"}
    weight = equal_weight_percent(4, 0.05, 10)
    assert weight == 0.95 / 4
    assert 4 * weight <= 0.95 + 1e-12
    assert_targets_sized(portfolio, weight, sorted(buys))
    assert_at_most_max_positions(portfolio, max_positions=10)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=0.05)
    close = order_close(portfolio, "VZ")
    capped = expected_shares(100_000, equal_weight_percent(10, 0.05, 10), close)
    assert buys["VZ"] > capped
    assert namespace_server.get("rsi")


def test_place_orders_respects_max_positions(session: Session, namespace_server):
    assets, as_of = _book(session)
    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="GOOGL", amount=10)])
    place_orders(assets, portfolio, max_positions=2, cash_reserve=0.05)

    buys = set(positive_targets(portfolio))
    assert buys == {"VZ", "MCD"}
    assert_flat(portfolio, ["GOOGL", "MMM", "TSM"])
    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["GOOGL"] == -10
    assert "MMM" not in orders
    assert "TSM" not in orders
    weight = equal_weight_percent(2, 0.05, 2)
    assert_targets_sized(portfolio, weight, sorted(buys))
    assert_at_most_max_positions(portfolio, max_positions=2)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=0.05)
    assert namespace_server.get("rsi")
