from foreverbull_cli.usercopy import SKIPPED_MISSED_CLOSE_STATUS
from foreverbull_cli.usercopy import LaterTicker
from foreverbull_cli.usercopy import backtest_starts_before_calendar
from foreverbull_cli.usercopy import backtest_starts_before_calendar_file
from foreverbull_cli.usercopy import backtest_starts_before_calendar_flag
from foreverbull_cli.usercopy import deployment_run_error
from foreverbull_cli.usercopy import later_ticker_lines
from foreverbull_cli.usercopy import later_tickers

from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb


# The exact lines, one place to update when the wording changes.
FB_FROM_META = "Prices for FB come from META, the company's later ticker."
TWTR_FROM_X = "Prices for TWTR come from X, the company's later ticker."
ONE_MORE = "Plus 1 more symbol whose prices come from a later ticker."
TWO_MORE = "Plus 2 more symbols whose prices come from a later ticker."

FB = LaterTicker(symbol="FB", read_as="META")
TWTR = LaterTicker(symbol="TWTR", read_as="X")


def _many(count: int) -> list[LaterTicker]:
    return [LaterTicker(symbol=f"S{i}", read_as=f"T{i}") for i in range(count)]


FIRST_FIVE = [f"Prices for S{i} come from T{i}, the company's later ticker." for i in range(5)]


_BROKER_START = (
    "This backtest starts on 1989-12-31, before the first trading day in "
    "foreverbull's market calendar (1990-01-02), so it can't be created. "
    "Set the start to 1990-01-02 or later, then create the backtest again."
)
_FLAG_START = (
    "This backtest starts on 1989-12-31, before the first trading day in "
    "foreverbull's market calendar (1990-01-02), so it can't be created. "
    "Run `fbull backtest create` again with --start 1990-01-02 or later."
)
_FILE_START = (
    "This backtest starts on 1989-12-31, before the first trading day in "
    "foreverbull's market calendar (1990-01-02), so it can't be created. "
    "Set start to 1990-01-02 or later in the backtest file, then run "
    "`fbull backtest create` again."
)


def test_start_before_calendar_with_start_flag_is_the_flag_sentence():
    assert backtest_starts_before_calendar_flag("1989-12-31", "1990-01-02") == _FLAG_START
    assert backtest_starts_before_calendar(_BROKER_START, start_flag=True) == _FLAG_START


def test_start_before_calendar_from_a_file_is_the_file_sentence():
    assert backtest_starts_before_calendar_file("1989-12-31", "1990-01-02") == _FILE_START
    assert backtest_starts_before_calendar(_BROKER_START, start_flag=False) == _FILE_START


def test_skipped_missed_close_is_the_status_sentence():
    assert deployment_run_error("skipped_missed_close") == SKIPPED_MISSED_CLOSE_STATUS
    assert SKIPPED_MISSED_CLOSE_STATUS == "Missed the close. Skipped because of --skip-missed."


def test_other_deployment_run_errors_pass_through():
    assert deployment_run_error("broker down") == "broker down"
    assert deployment_run_error("") == ""
    assert deployment_run_error("skipped_missed_close ") == "skipped_missed_close "


def test_no_later_ticker_prints_nothing():
    assert later_ticker_lines([]) == []


def test_one_later_ticker_is_one_line():
    assert later_ticker_lines([FB]) == [FB_FROM_META]


def test_several_later_tickers_are_one_line_each():
    assert later_ticker_lines([FB, TWTR]) == [FB_FROM_META, TWTR_FROM_X]


def test_five_later_tickers_print_no_count_line():
    assert later_ticker_lines(_many(5)) == FIRST_FIVE


def test_one_past_five_is_counted_as_one_symbol():
    assert later_ticker_lines(_many(6)) == [*FIRST_FIVE, ONE_MORE]


def test_more_past_five_are_counted_as_symbols():
    assert later_ticker_lines(_many(7)) == [*FIRST_FIVE, TWO_MORE]


def test_later_tickers_read_the_stored_resolutions_in_order():
    entity = backtest_pb.Backtest(
        name="oef",
        ticker_resolutions=[
            backtest_pb.Backtest.TickerResolution(symbol="FB", read_as="META"),
            backtest_pb.Backtest.TickerResolution(symbol="TWTR", read_as="X"),
        ],
    )
    assert later_tickers(entity) == [FB, TWTR]


def test_a_backtest_without_resolutions_has_none():
    assert later_tickers(backtest_pb.Backtest(name="oef")) == []
