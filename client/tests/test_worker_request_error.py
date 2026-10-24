from sqlalchemy.exc import ProgrammingError

from foreverbull.models.finance import WORKER_MARKET_DATA_DENIED
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
    assert "SELECT" not in text
    assert "asset_symbol" not in text
    assert "SQLAlchemy" not in text
    assert "permission denied for table" not in text


def test_pgcode_permission_denied_uses_the_same_sentence():
    orig = _DatabaseError("permission denied for table asset_symbol", pgcode="42501")
    err = ProgrammingError("SELECT 1 FROM asset_symbol", {}, orig)
    assert worker_request_error(err) == WORKER_MARKET_DATA_DENIED


def test_other_errors_keep_their_text():
    err = ValueError("cash reserve outside [0, 1)")
    assert worker_request_error(err) == repr(err)
