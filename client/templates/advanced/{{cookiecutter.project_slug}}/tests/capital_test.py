import pytest

from allocation import equal_weight_percent
from macd import calculate_macd
from macd import handle_data as macd_handle_data
from macd import select_positive_macd
from rsi import place_orders
from rsi import select_oversold

from foreverbull_testing.assertions import assert_at_most_max_positions
from foreverbull_testing.assertions import assert_positive_weights_within_reserve
from foreverbull_testing.assertions import assert_targets_sized
from foreverbull_testing.assertions import expected_shares
from foreverbull_testing.assertions import long_notional
from foreverbull_testing.assertions import order_close
from foreverbull_testing.assertions import parameter_default
from foreverbull_testing.assertions import positive_targets
from foreverbull_testing.session import Session


# Injected RSI. Names under 30 are the book; AAPL is omitted so this file does not
# replace downloaded AAPL bars used by the indicator test.
METRICS = {
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
PRICES = {symbol: 25.0 + index * 10.0 for index, symbol in enumerate(METRICS)}


def _rsi_book(session: Session):
    times = None
    for symbol, price in PRICES.items():
        times = session.constant(symbol, 5, price)
    assert times is not None
    assets = session.assets(list(PRICES), times[-1])
    assets.set_metrics("rsi", dict(METRICS))
    return assets, times[-1]


def _macd_book(session: Session):
    steep = session.trend("STEEP", 60, 20.0, 80.0)
    session.trend("MILD", 60, 20.0, 40.0)
    session.trend("SLOW", 60, 30.0, 45.0)
    as_of = steep[-1]
    assets = session.assets(["STEEP", "MILD", "SLOW"], as_of)
    return assets, as_of


def _macd_selected(assets, max_positions: int) -> list[str]:
    df = assets.stock_data.groupby(level="symbol", group_keys=False).apply(calculate_macd)
    latest = df.groupby(level="symbol", group_keys=False).apply(lambda frame: frame.iloc[-1]["macd"])
    return select_positive_macd(latest, max_positions)


def test_rsi_place_orders_deploys_capital(session: Session, namespace_server):
    assets, as_of = _rsi_book(session)
    portfolio = session.portfolio(as_of, 100_000)
    place_orders(assets, portfolio)

    cash_reserve = parameter_default(place_orders, "cash_reserve")
    max_positions = parameter_default(place_orders, "max_positions")
    selected = select_oversold(assets.get_metrics("rsi"), parameter_default(place_orders, "oversold"), max_positions)
    assert selected
    assert_at_most_max_positions(portfolio, fn=place_orders)
    assert_positive_weights_within_reserve(portfolio, fn=place_orders)
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_targets_sized(portfolio, weight, selected)
    assert namespace_server.get("rsi")


def test_rsi_place_orders_scales_with_portfolio_value(session: Session, namespace_server):
    assets, as_of = _rsi_book(session)
    small = session.portfolio(as_of, 100_000)
    large = session.portfolio(as_of, 400_000)
    place_orders(assets, small)
    place_orders(assets, large)

    max_positions = parameter_default(place_orders, "max_positions")
    cash_reserve = parameter_default(place_orders, "cash_reserve")
    selected = select_oversold(assets.get_metrics("rsi"), parameter_default(place_orders, "oversold"), max_positions)
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_targets_sized(small, weight, selected)
    assert_targets_sized(large, weight, selected)
    assert_positive_weights_within_reserve(small, fn=place_orders)
    assert_positive_weights_within_reserve(large, fn=place_orders)
    assert long_notional(large) / long_notional(small) == pytest.approx(4.0, rel=0.10)
    assert namespace_server.get("rsi")


def test_rsi_place_orders_concentrated_book_still_deploys_capital(session: Session, namespace_server):
    assets, as_of = _rsi_book(session)
    portfolio = session.portfolio(as_of, 100_000)
    place_orders(assets, portfolio)

    buys = set(positive_targets(portfolio))
    assert buys == {"VZ", "MCD", "MMM", "TSM"}
    cash_reserve = parameter_default(place_orders, "cash_reserve")
    max_positions = parameter_default(place_orders, "max_positions")
    weight = equal_weight_percent(len(buys), cash_reserve, max_positions)
    capped = equal_weight_percent(max_positions, cash_reserve, max_positions)
    assert weight > capped
    assert_targets_sized(portfolio, weight, sorted(buys))
    assert_positive_weights_within_reserve(portfolio, fn=place_orders)
    assert_at_most_max_positions(portfolio, fn=place_orders)
    for symbol in buys:
        close = order_close(portfolio, symbol)
        assert positive_targets(portfolio)[symbol] > expected_shares(100_000, capped, close)
    assert namespace_server.get("rsi")


def test_macd_handle_data_deploys_capital(session: Session):
    assets, as_of = _macd_book(session)
    portfolio = session.portfolio(as_of, 100_000)
    macd_handle_data(assets, portfolio)

    max_positions = parameter_default(macd_handle_data, "max_positions")
    cash_reserve = parameter_default(macd_handle_data, "cash_reserve")
    selected = _macd_selected(assets, max_positions)
    assert selected
    assert_at_most_max_positions(portfolio, fn=macd_handle_data)
    assert_positive_weights_within_reserve(portfolio, fn=macd_handle_data)
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_targets_sized(portfolio, weight, selected)


def test_macd_handle_data_scales_with_portfolio_value(session: Session):
    assets, as_of = _macd_book(session)
    cash_reserve = 0.10
    max_positions = parameter_default(macd_handle_data, "max_positions")
    small = session.portfolio(as_of, 100_000)
    large = session.portfolio(as_of, 400_000)
    macd_handle_data(assets, small, max_positions=max_positions, cash_reserve=cash_reserve)
    macd_handle_data(assets, large, max_positions=max_positions, cash_reserve=cash_reserve)

    selected = _macd_selected(assets, max_positions)
    assert selected
    weight = equal_weight_percent(len(selected), cash_reserve, max_positions)
    assert_targets_sized(small, weight, selected)
    assert_targets_sized(large, weight, selected)
    assert_positive_weights_within_reserve(small, cash_reserve=cash_reserve)
    assert_positive_weights_within_reserve(large, cash_reserve=cash_reserve)
    assert long_notional(small) > 0
    assert long_notional(large) / long_notional(small) == pytest.approx(4.0, rel=0.10)
