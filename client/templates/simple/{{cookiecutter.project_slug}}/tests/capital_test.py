import pytest

from crossover import get_means
from crossover import handle_data

from foreverbull_testing.assertions import assert_at_most_max_positions
from foreverbull_testing.assertions import assert_flat
from foreverbull_testing.assertions import assert_not_magic_lot
from foreverbull_testing.assertions import assert_positive_weights_within_reserve
from foreverbull_testing.assertions import assert_targets_sized
from foreverbull_testing.assertions import long_notional
from foreverbull_testing.models import Position
from foreverbull_testing.session import Session


CASH_RESERVE = 0.05
MAX_POSITIONS = 10


def test_handle_data_deploys_capital(session: Session):
    times = session.cross_up("XUP", 10, 30)
    as_of = times[-1]
    assets = session.assets(["XUP"], as_of)
    short_mean, long_mean = get_means(next(iter(assets)))
    assert short_mean > long_mean

    portfolio = session.portfolio(as_of, 100_000)
    handle_data(assets, portfolio)

    assert_not_magic_lot(portfolio)
    weight = (1.0 - CASH_RESERVE) / 1
    assert_at_most_max_positions(portfolio, max_positions=MAX_POSITIONS)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=CASH_RESERVE)
    assert_targets_sized(portfolio, weight, ["XUP"])


def test_handle_data_scales_with_portfolio_value(session: Session):
    times = session.cross_up("XUP", 10, 30)
    as_of = times[-1]
    assets = session.assets(["XUP"], as_of)
    short_mean, long_mean = get_means(next(iter(assets)))
    assert short_mean > long_mean

    small = session.portfolio(as_of, 100_000)
    large = session.portfolio(as_of, 400_000)
    handle_data(assets, small)
    handle_data(assets, large)

    assert_not_magic_lot(small)
    assert_not_magic_lot(large)
    weight = 1.0 - CASH_RESERVE
    assert_targets_sized(small, weight, ["XUP"])
    assert_targets_sized(large, weight, ["XUP"])
    assert_positive_weights_within_reserve(small, cash_reserve=CASH_RESERVE)
    assert_positive_weights_within_reserve(large, cash_reserve=CASH_RESERVE)
    assert long_notional(large) / long_notional(small) == pytest.approx(4.0, rel=0.10)


def test_handle_data_not_magic_lot(session: Session):
    times = session.cross_up("XUP", 10, 30)
    session.trend("XDN", len(times), 50.0, 10.0)
    as_of = times[-1]
    assets = session.assets(["XUP", "XDN"], as_of)
    by_symbol = {asset.symbol: asset for asset in assets}
    up_short, up_long = get_means(by_symbol["XUP"])
    down_short, down_long = get_means(by_symbol["XDN"])
    assert up_short > up_long
    assert down_short < down_long

    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="XDN", amount=10)])
    handle_data(assets, portfolio)

    assert_flat(portfolio, ["XDN"])
    assert_not_magic_lot(portfolio)
    assert_at_most_max_positions(portfolio, max_positions=MAX_POSITIONS)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=CASH_RESERVE)
    assert_targets_sized(portfolio, 1.0 - CASH_RESERVE, ["XUP"])
