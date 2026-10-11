from inspect import signature

import pandas as pd

from allocation import equal_weight_percent
from macd import algo
from macd import calculate_macd
from macd import handle_data
from macd import select_positive_macd

from foreverbull_testing.assertions import assert_at_most_max_positions
from foreverbull_testing.assertions import assert_flat
from foreverbull_testing.assertions import assert_positive_weights_within_reserve
from foreverbull_testing.assertions import assert_targets_sized
from foreverbull_testing.assertions import parameter_default
from foreverbull_testing.models import Position
from foreverbull_testing.session import Session


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
    df = assets.stock_data.groupby(level="symbol", group_keys=False).apply(
        calculate_macd,
        fastperiod=parameter_default(handle_data, "fastperiod"),
        slowperiod=parameter_default(handle_data, "slowperiod"),
        signalperiod=parameter_default(handle_data, "signalperiod"),
    )
    latest = df.groupby(level="symbol", group_keys=False).apply(lambda frame: frame.iloc[-1]["macd"])
    return select_positive_macd(latest, max_positions)


def _trends(session: Session):
    steep = session.trend("STEEP", 60, 20.0, 80.0)
    session.trend("MILD", 60, 20.0, 40.0)
    session.trend("FALL", 60, 80.0, 20.0)
    return steep[-1]


def test_handle_data_rebalances_held_and_flattens_leavers(session: Session):
    as_of = _trends(session)
    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="FALL", amount=10)])
    assets = session.assets(["STEEP", "MILD", "FALL"], as_of)
    handle_data(assets, portfolio)

    selected = _selected(assets)
    assert "STEEP" in selected
    assert "FALL" not in selected
    assert_flat(portfolio, ["FALL"])
    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["FALL"] == -10

    cash_reserve = parameter_default(handle_data, "cash_reserve")
    max_positions = parameter_default(handle_data, "max_positions")
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_at_most_max_positions(portfolio, fn=handle_data)
    assert_positive_weights_within_reserve(portfolio, fn=handle_data)
    assert_targets_sized(portfolio, weight, selected)
    assert orders["STEEP"] != 10


def test_handle_data_flattens_when_max_positions_zero(session: Session):
    times = session.constant("HELD", 60, 100.0)
    session.constant("OTHER", 60, 100.0)
    portfolio = session.portfolio(times[-1], 100_000, positions=[Position(symbol="HELD", amount=10)])
    assets = session.assets(["HELD", "OTHER"], times[-1])

    handle_data(assets, portfolio, max_positions=0, cash_reserve=0.05)

    assert_flat(portfolio, ["HELD", "OTHER"])
    assert_at_most_max_positions(portfolio, max_positions=0)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=0.05)
    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["HELD"] == -10
    assert all(amount <= 0 for amount in orders.values())


def test_handle_data_respects_max_positions(session: Session):
    as_of = _trends(session)
    assets = session.assets(["STEEP", "MILD"], as_of)
    portfolio = session.portfolio(as_of, 100_000)
    handle_data(assets, portfolio, max_positions=1, cash_reserve=0.10)

    selected = _selected(assets, max_positions=1)
    assert selected == ["STEEP"]
    assert_at_most_max_positions(portfolio, max_positions=1)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=0.10)
    assert equal_weight_percent(1, 0.10, 1) == 0.90
    assert_targets_sized(portfolio, 0.90, selected)
