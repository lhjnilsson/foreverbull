from sqlalchemy.exc import ProgrammingError

from foreverbull.models.finance import WORKER_MARKET_DATA_DENIED
from foreverbull.models.finance import market_data_denied_line
from foreverbull.models.finance import present_market_data_failure
from foreverbull.models.finance import worker_request_error


class _DatabaseError(Exception):
    def __init__(self, message: str, *, sqlstate: str = "", pgcode: str = ""):
        super().__init__(message)
        self.sqlstate = sqlstate
        self.pgcode = pgcode


def test_permission_denied_is_one_sentence_without_the_statement():
    statement = "SELECT symbol FROM asset_symbol WHERE symbol IN ('AAPL')"
    orig = _DatabaseError("permission denied for table asset_symbol", sqlstate="42501")
    err = ProgrammingError(statement, {}, orig)
    text = worker_request_error(err)
    assert text == WORKER_MARKET_DATA_DENIED
    assert text == market_data_denied_line(None)
    assert "SELECT" not in text
    assert "asset_symbol" not in text
    assert "SQLAlchemy" not in text
    assert "permission denied for table" not in text
    assert "worker" not in text.lower()
    assert "database" not in text.lower()
    assert "table" not in text.lower()


def test_pgcode_permission_denied_uses_the_same_sentence():
    orig = _DatabaseError("permission denied for table asset_symbol", pgcode="42501")
    err = ProgrammingError("SELECT 1 FROM asset_symbol", {}, orig)
    assert worker_request_error(err) == WORKER_MARKET_DATA_DENIED


def test_other_errors_keep_their_text():
    err = ValueError("cash reserve outside [0, 1)")
    assert worker_request_error(err) == repr(err)


def test_the_printed_line_names_the_backtest_when_the_command_has_one():
    named = "Couldn't run backtest 'spy': the data service can't read market data. This is a bug in the Foreverbull install, not in your strategy."
    assert market_data_denied_line("spy") == named
    assert market_data_denied_line("  spy  ") == named
    assert market_data_denied_line("") == WORKER_MARKET_DATA_DENIED
    assert market_data_denied_line("   ") == WORKER_MARKET_DATA_DENIED
    wrapped = "run execution: rpc error: code = FailedPrecondition desc = " + WORKER_MARKET_DATA_DENIED
    assert present_market_data_failure(wrapped, "spy") == named
    assert present_market_data_failure("some other failure", "spy") == "some other failure"
