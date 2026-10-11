import pytest

from allocation import deployable_value
from allocation import equal_weight_percent
from allocation import equal_weight_targets


def test_deployable_value():
    assert deployable_value(100_000, 0.05) == 95_000
    assert deployable_value(100_000, 0.0) == 100_000


@pytest.mark.parametrize(
    "n_selected,cash_reserve,max_positions,expected",
    [
        (10, 0.05, 10, 0.095),
        (3, 0.05, 10, 0.95 / 3),
        (50, 0.10, 10, 0.09),
        (0, 0.05, 10, 0.0),
    ],
)
def test_equal_weight_percent(n_selected, cash_reserve, max_positions, expected):
    weight = equal_weight_percent(n_selected, cash_reserve, max_positions)
    assert weight == pytest.approx(expected)
    n = min(max_positions, n_selected)
    if n >= 1:
        assert n * weight == pytest.approx(1.0 - cash_reserve)
        assert n * weight <= 1.0 - cash_reserve + 1e-12


def test_equal_weight_targets_concentrates_when_few_qualify():
    targets = equal_weight_targets(["VZ", "MCD", "MMM"], 0.05, 10)
    assert list(targets) == ["VZ", "MCD", "MMM"]
    assert sum(targets.values()) == pytest.approx(0.95)
    assert targets["VZ"] == pytest.approx(0.95 / 3)


def test_equal_weight_targets_caps_at_max_positions():
    targets = equal_weight_targets(["A", "B", "C", "D"], 0.10, 2)
    assert list(targets) == ["A", "B"]
    assert sum(targets.values()) == pytest.approx(0.90)
