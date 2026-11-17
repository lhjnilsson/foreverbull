import io
import os

import pandas
import pytest

from foreverbull_testing.models import BacktestResult

from foreverbull.broker.backtest import RESULT_MAX_BYTES
from foreverbull.broker.backtest import _join_result
from foreverbull.exceptions import ResultError
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb


def _clear_storage_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in list(os.environ):
        if name.startswith("STORAGE_"):
            monkeypatch.delenv(name, raising=False)


def _result(execution_id: str = "01234567-89ab-cdef-0123-456789abcdef") -> BacktestResult:
    return BacktestResult(backtest_pb.Backtest(name="spy"), execution_id)


def _pickle(frame: pandas.DataFrame) -> bytes:
    buf = io.BytesIO()
    frame.to_pickle(buf)
    return buf.getvalue()


def _refuse_storage(*_args, **_kwargs):
    raise AssertionError("Storage.from_environment was called")


def test_dataframe_reads_the_broker_without_storage_env(monkeypatch: pytest.MonkeyPatch) -> None:
    _clear_storage_env(monkeypatch)
    frame = pandas.DataFrame({"portfolio_value": [100.0, 101.5]})
    calls: list[str] = []

    def fetch(execution_id: str) -> bytes:
        calls.append(execution_id)
        return _pickle(frame)

    monkeypatch.setattr("foreverbull_testing.models.get_execution_result", fetch)
    monkeypatch.setattr("foreverbull.broker.storage.Storage.from_environment", _refuse_storage)

    result = _result()
    loaded = result.dataframe
    again = result.dataframe

    pandas.testing.assert_frame_equal(loaded, frame)
    assert again is loaded
    assert calls == ["01234567-89ab-cdef-0123-456789abcdef"]
    assert not any(name.startswith("STORAGE_") for name in os.environ)


def test_dataframe_failure_names_the_execution_and_does_not_try_storage(monkeypatch: pytest.MonkeyPatch) -> None:
    _clear_storage_env(monkeypatch)

    class BrokerFailure(Exception):
        def details(self) -> str:
            return "This execution has no stored result."

    def fetch(_execution_id: str) -> bytes:
        raise BrokerFailure("status")

    monkeypatch.setattr("foreverbull_testing.models.get_execution_result", fetch)
    monkeypatch.setattr("foreverbull.broker.storage.Storage.from_environment", _refuse_storage)

    with pytest.raises(
        ResultError,
        match="Couldn't read the result for execution exec-9. This execution has no stored result.",
    ) as raised:
        _ = _result("exec-9").dataframe

    assert "STORAGE_" not in str(raised.value)
    assert "minio" not in str(raised.value)


def test_dataframe_refuses_a_non_dataframe(monkeypatch: pytest.MonkeyPatch) -> None:
    buf = io.BytesIO()
    pandas.Series([1, 2]).to_pickle(buf)
    monkeypatch.setattr("foreverbull_testing.models.get_execution_result", lambda _execution_id: buf.getvalue())

    with pytest.raises(ResultError, match="The broker's answer is not a dataframe."):
        _ = _result().dataframe


def test_join_result_refuses_an_empty_or_oversized_stream() -> None:
    assert _join_result("exec-1", [b"ab", b"", b"c"]) == b"abc"
    with pytest.raises(ResultError, match="The broker returned an empty result."):
        _join_result("exec-1", [b"", b""])
    with pytest.raises(ResultError, match="too large to return"):
        _join_result("exec-1", [b"x" * (RESULT_MAX_BYTES + 1)])
