import pytest

from foreverbull_testing.assertions import assert_at_most_max_positions
from foreverbull_testing.assertions import assert_expected_shares
from foreverbull_testing.assertions import assert_flat
from foreverbull_testing.assertions import assert_not_magic_lot
from foreverbull_testing.assertions import assert_positive_weights_within_reserve
from foreverbull_testing.assertions import assert_targets_sized
from foreverbull_testing.assertions import expected_shares
from foreverbull_testing.assertions import parameter_default
from foreverbull_testing.models import Position
from foreverbull_testing.session import Session


class _PB:
    def __init__(self, portfolio_value: float):
        self.portfolio_value = portfolio_value


class _Pos:
    def __init__(self, symbol: str, amount: int):
        self.symbol = symbol
        self.amount = amount


class _Order:
    def __init__(self, symbol: str, amount: int):
        self.symbol = symbol
        self.amount = amount


class _Book:
    def __init__(self, value: float, positions: list[_Pos], orders: list[_Order], closes: dict[str, float]):
        self._pb = _PB(value)
        self._positions = positions
        self.pending_orders = orders
        self._closes = closes

    @property
    def positions(self) -> list[_Pos]:
        return list(self._positions)

    def _close_on_bar(self, symbol: str) -> float:
        return self._closes[symbol]


def _sized(asset, portfolio, max_positions: int = 2, cash_reserve: float = 0.25):
    return asset, portfolio


def _bare(asset, portfolio):
    return asset, portfolio


def test_expected_shares_matches_worker_formula():
    assert expected_shares(100_000, 0.1, 30) == int(100_000 * 0.1 / 30)
    assert expected_shares(100_000, 0.0, 30) == 0


def test_share_count_allows_one_share():
    expected = expected_shares(100_000, 0.1, 30)
    assert_expected_shares(expected + 1, 100_000, 0.1, 30)
    assert_expected_shares(expected - 1, 100_000, 0.1, 30)
    with pytest.raises(AssertionError):
        assert_expected_shares(expected + 2, 100_000, 0.1, 30)


def test_targets_sized_includes_the_carried_position():
    book = _Book(100_000, [_Pos("A", 10)], [_Order("A", expected_shares(100_000, 0.1, 30) - 10)], {"A": 30.0})
    assert_targets_sized(book, 0.1, ["A"])
    book.pending_orders = [_Order("A", expected_shares(100_000, 0.1, 30) - 10 + 2)]
    with pytest.raises(AssertionError):
        assert_targets_sized(book, 0.1, ["A"])


def test_weight_sum_uses_the_order_close():
    # 100 shares at the order close of 10 is half of a 2000 book.
    # A close of 1 would hide that weight under a 0.6 reserve.
    book = _Book(2000, [], [_Order("AAA", 100)], {"AAA": 10.0})
    assert_positive_weights_within_reserve(book, cash_reserve=0.4)
    with pytest.raises(AssertionError, match="positive target weights"):
        assert_positive_weights_within_reserve(book, cash_reserve=0.6)


def test_omitted_limits_come_from_the_callable():
    book = _Book(1000, [], [_Order("A", 1), _Order("B", 1)], {"A": 50.0, "B": 50.0})
    assert parameter_default(_sized, "max_positions") == 2
    assert parameter_default(_sized, "cash_reserve") == 0.25
    assert_at_most_max_positions(book, fn=_sized)
    assert_positive_weights_within_reserve(book, fn=_sized)
    with pytest.raises(AssertionError):
        assert_at_most_max_positions(book, max_positions=1)
    with pytest.raises(AssertionError):
        assert_positive_weights_within_reserve(book, cash_reserve=0.95, fn=_sized)


def test_explicit_zero_is_not_replaced_by_the_callable_default():
    empty = _Book(1000, [], [], {})
    held = _Book(1000, [], [_Order("A", 1)], {"A": 50.0})
    assert_at_most_max_positions(empty, 0, fn=_sized)
    with pytest.raises(AssertionError):
        assert_at_most_max_positions(held, 0, fn=_sized)
    # Sum of weights is 0.05. Reserve 0 leaves room; the callable default of 0.25 is not required.
    assert_positive_weights_within_reserve(held, 0.0, fn=_sized)


def test_missing_limit_without_a_default_errors():
    book = _Book(1000, [], [], {})
    with pytest.raises(TypeError, match="max_positions"):
        assert_at_most_max_positions(book)
    with pytest.raises(TypeError, match="cash_reserve"):
        assert_positive_weights_within_reserve(book)
    with pytest.raises(TypeError, match="no parameter"):
        assert_at_most_max_positions(book, fn=_bare)


def test_flat_targets_are_zero_including_names_that_were_never_held():
    book = _Book(
        1000, [_Pos("A", 10), _Pos("B", 4)], [_Order("A", -10), _Order("C", 3)], {"A": 1.0, "B": 1.0, "C": 1.0}
    )
    assert_flat(book, ["A", "D"])
    with pytest.raises(AssertionError, match="B target is 4"):
        assert_flat(book, ["B"])


def test_magic_lot_rejects_only_an_all_ten_share_book():
    tens = _Book(100_000, [], [_Order("A", 10), _Order("B", 10)], {"A": 20.0, "B": 20.0})
    mixed = _Book(100_000, [], [_Order("A", 10), _Order("B", 12)], {"A": 20.0, "B": 20.0})
    with pytest.raises(AssertionError, match="magic 10-share lot"):
        assert_not_magic_lot(tens)
    assert_not_magic_lot(mixed)


def test_sized_and_flat_against_worker_portfolio(session: Session):
    times = session.constant("CAP", 1, 30.0)
    as_of = times[-1]
    portfolio = session.portfolio(as_of, 100_000, positions=[Position(symbol="CAP", amount=5)])
    portfolio.order_target_percent("CAP", 0.10)
    assert_targets_sized(portfolio, 0.10, ["CAP"])
    assert_at_most_max_positions(portfolio, max_positions=1)
    assert_positive_weights_within_reserve(portfolio, cash_reserve=0.5)

    flat = session.portfolio(as_of, 100_000, positions=[Position(symbol="CAP", amount=5)])
    flat.order_target("CAP", 0)
    assert_flat(flat, ["CAP"])
