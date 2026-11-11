"""Exact lines for a universe-drop close that cannot be filled."""

from datetime import date

from foreverbull_zipline.universe_close import CloseFailure
from foreverbull_zipline.universe_close import close_failure_line


_ISSUES = "https://github.com/lhjnilsson/foreverbull/issues"


def test_no_close_price_names_one_symbol_and_session():
    line = close_failure_line([CloseFailure("ABC", date(2023, 1, 31), "no_price")])
    assert line == (
        "foreverbull couldn't close ABC at the close on 2023-01-31 because there was no close price that session. "
        "The run was stopped. Check the prices for ABC on 2023-01-31, "
        f"or report it at {_ISSUES} with this message."
    )


def test_no_close_price_names_every_symbol_on_one_session():
    line = close_failure_line(
        [
            CloseFailure("DEF", date(2023, 1, 31), "no_price"),
            CloseFailure("ABC", date(2023, 1, 31), "no_price"),
        ]
    )
    assert line == (
        "foreverbull couldn't close ABC and DEF at the close on 2023-01-31 because there was no close price that session. "
        "The run was stopped. Check the prices for ABC and DEF on 2023-01-31, "
        f"or report it at {_ISSUES} with this message."
    )


def test_no_close_price_names_each_session():
    line = close_failure_line(
        [
            CloseFailure("DEF", date(2023, 2, 28), "no_price"),
            CloseFailure("ABC", date(2023, 1, 31), "no_price"),
        ]
    )
    assert line == (
        "foreverbull couldn't close ABC at the close on 2023-01-31 and DEF at the close on 2023-02-28 "
        "because there was no close price for those sessions. "
        "The run was stopped. Check the prices for ABC on 2023-01-31 and DEF on 2023-02-28, "
        f"or report it at {_ISSUES} with this message."
    )


def test_rejected_closes_name_three_symbols_on_one_session():
    line = close_failure_line(
        [
            CloseFailure("GHI", date(2023, 1, 31), "rejected"),
            CloseFailure("ABC", date(2023, 1, 31), "rejected"),
            CloseFailure("DEF", date(2023, 1, 31), "rejected"),
        ]
    )
    assert line == (
        "foreverbull couldn't close ABC, DEF and GHI at the close on 2023-01-31 because the closes were rejected. "
        "This is a problem in foreverbull, not in your algo. The run was stopped. "
        f"Please report it at {_ISSUES} with this message."
    )


def test_rejected_closes_name_each_session():
    line = close_failure_line(
        [
            CloseFailure("DEF", date(2023, 2, 28), "rejected"),
            CloseFailure("ABC", date(2023, 1, 31), "rejected"),
        ]
    )
    assert line == (
        "foreverbull couldn't close ABC at the close on 2023-01-31 and DEF at the close on 2023-02-28 "
        "because the closes were rejected. "
        "This is a problem in foreverbull, not in your algo. The run was stopped. "
        f"Please report it at {_ISSUES} with this message."
    )


def test_a_rejected_close_names_the_symbol_and_session():
    line = close_failure_line([CloseFailure("ABC", date(2023, 1, 31), "rejected")])
    assert line == (
        "foreverbull couldn't close ABC at the close on 2023-01-31 because the close was rejected. "
        "This is a problem in foreverbull, not in your algo. The run was stopped. "
        f"Please report it at {_ISSUES} with this message."
    )


def test_mixed_causes_name_each_symbol_and_session():
    line = close_failure_line(
        [
            CloseFailure("DEF", date(2023, 2, 28), "rejected"),
            CloseFailure("ABC", date(2023, 1, 31), "no_price"),
        ]
    )
    assert line == (
        "foreverbull couldn't close ABC at the close on 2023-01-31 because there was no close price that session, "
        "and it couldn't close DEF at the close on 2023-02-28 because the close was rejected. "
        "The run was stopped. "
        f"Please report it at {_ISSUES} with this message."
    )
