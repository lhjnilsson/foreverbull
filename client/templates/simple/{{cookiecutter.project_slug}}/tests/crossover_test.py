from inspect import signature

import numpy

from crossover import algo
from crossover import get_means
from crossover import handle_data

from foreverbull_testing.assertions import assert_at_most_max_positions
from foreverbull_testing.assertions import assert_flat
from foreverbull_testing.assertions import assert_not_magic_lot
from foreverbull_testing.assertions import assert_targets_sized
from foreverbull_testing.assertions import positive_targets
from foreverbull_testing.models import Backtest
from foreverbull_testing.models import Position
from foreverbull_testing.session import Session


def test_get_means(session: Session):
    times = session.cross_up("SMA", 10, 30)
    asset = next(iter(session.assets(["SMA"], times[-1])))
    short_mean, long_mean = get_means(asset)
    assert type(short_mean) is numpy.float64
    assert type(long_mean) is numpy.float64
    assert short_mean > 0
    assert long_mean > 0
    assert short_mean > long_mean


def test_function_parameters():
    params = signature(handle_data).parameters
    assert params["short_window"].default == 10
    assert params["long_window"].default == 30
    assert params["cash_reserve"].default == 0.05
    assert params["max_positions"].default == 10

    fn = algo.get_definition().functions[0]
    assert fn.name == "handle_data"
    assert [parameter.key for parameter in fn.parameters] == [
        "short_window",
        "long_window",
        "cash_reserve",
        "max_positions",
    ]
    assert [parameter.defaultValue for parameter in fn.parameters] == ["10", "30", "0.05", "10"]
    assert [parameter.valueType for parameter in fn.parameters] == ["int", "int", "float", "int"]


def test_handle_data_rebalances_keepers(session: Session):
    times = session.cross_up("SMA", 10, 30)
    as_of = times[-1]
    assets = session.assets(["SMA"], as_of)
    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="SMA", amount=10)])
    handle_data(assets, portfolio)

    assert_not_magic_lot(portfolio)
    assert_targets_sized(portfolio, 0.95, ["SMA"])
    orders = {order.symbol: order.amount for order in portfolio.pending_orders}
    assert orders["SMA"] != 10


def test_handle_data_respects_max_positions(session: Session):
    steep = session.trend("STEEP", 40, 20.0, 80.0)
    session.trend("MILD", 40, 20.0, 40.0)
    as_of = steep[-1]
    assets = session.assets(["MILD", "STEEP"], as_of)
    portfolio = session.portfolio(as_of, 100_000)
    handle_data(assets, portfolio, max_positions=1, cash_reserve=0.05)

    assert list(positive_targets(portfolio)) == ["STEEP"]
    assert_flat(portfolio, ["MILD"])
    assert_at_most_max_positions(portfolio, max_positions=1)
    assert_targets_sized(portfolio, 0.95, ["STEEP"])


def test_handle_data_drops_flat_or_short_history(session: Session):
    flat = session.constant("FLAT", 40, 50.0)
    session.constant("NEW", 5, 50.0)
    as_of = flat[-1]
    assets = session.assets(["FLAT", "NEW"], as_of)
    portfolio = session.portfolio(
        as_of,
        100_000,
        positions=[Position(symbol="FLAT", amount=4), Position(symbol="NEW", amount=4)],
    )
    handle_data(assets, portfolio)

    assert_flat(portfolio, ["FLAT", "NEW"])
    assert list(positive_targets(portfolio)) == []


def test_backtest(backtest: Backtest):
    backtest.run_execution(algo, backtest.get_default())
