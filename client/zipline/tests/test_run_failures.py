import logging

from types import SimpleNamespace

import grpc
import pandas as pd
import pynng
import pytest

from foreverbull_zipline.engine import PLAIN_SENTENCE_TAG
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.engine import EngineRequestError
from foreverbull_zipline.engine import StopExcecution
from foreverbull_zipline.engine import raise_response_error
from foreverbull_zipline.errors import PlainSentence
from foreverbull_zipline.returns import DividendBook
from foreverbull_zipline.returns import UnpaidDividendError
from foreverbull_zipline.session_service import ENGINE_RESULT_ERROR_TAG
from foreverbull_zipline.session_service import SessionServiceServicer
from protobuf import merge_from_binary
from pynng.exceptions import nng as _nng

from foreverbull.pb.foreverbull import common_pb


UNPAID = (
    "The backtest run stopped: foreverbull didn't pay a dividend on its ex-date. "
    "This is a problem in foreverbull, not in your algo. "
    "Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."
)


class _Aborted(Exception):
    pass


class _Context:
    def __init__(self):
        self.code: grpc.StatusCode | None = None
        self.details: str | None = None

    def abort(self, code, details):
        self.code, self.details = code, details
        raise _Aborted


class _Algorithm:
    def __init__(self, error: Exception | None):
        self.error = error

    def run(self):
        if self.error is not None:
            raise self.error


def _engine() -> Engine:
    engine = Engine()
    engine.log = logging.getLogger(__name__)
    return engine


def test_a_dividend_unpaid_at_a_close_stops_the_run():
    ledger = SimpleNamespace(position_tracker=SimpleNamespace(_unpaid_dividends={1: [{"amount": 48.5}]}))
    packet = {"daily_perf": {}}
    with pytest.raises(UnpaidDividendError) as raised:
        DividendBook(set()).end_of_session(packet, ledger, pd.Timestamp("2024-03-14"), 0, None)
    assert str(raised.value) == (
        "dividends of 48.500000 were not paid on their ex-date by 2024-03-14: the bundle's pay date must be the ex-date"
    )


def test_an_unpaid_dividend_is_kept_as_the_runs_failure():
    engine = _engine()
    engine._run_algorithm(_Algorithm(UnpaidDividendError("dividends of 48.500000 were not paid")))
    assert isinstance(engine._run_failure, PlainSentence)
    assert str(engine._run_failure) == UNPAID


def test_a_mid_run_failure_is_kept_with_its_cause():
    engine = _engine()
    engine._run_algorithm(_Algorithm(ValueError("prices missing for META on 2024-03-14")))
    assert isinstance(engine._run_failure, PlainSentence)
    assert str(engine._run_failure) == "ValueError: prices missing for META on 2024-03-14"
    engine._run_algorithm(_Algorithm(None))
    assert engine._run_failure is None


def test_a_stop_during_the_run_is_not_stored_as_a_failure():
    engine = _engine()
    with pytest.raises(StopExcecution):
        engine._run_algorithm(_Algorithm(StopExcecution()))
    assert engine._run_failure is None


def test_the_period_request_after_a_named_failure_tells_it_to_the_broker():
    class _Stopped:
        def get_current_period(self, request):
            raise_response_error(common_pb.Response(task="get_current_period", error=PLAIN_SENTENCE_TAG + UNPAID))

    context = _Context()
    with pytest.raises(_Aborted):
        SessionServiceServicer(_Stopped()).get_current_period(None, context)  # type: ignore[arg-type]
    assert context.code == grpc.StatusCode.FAILED_PRECONDITION
    assert context.details == UNPAID


def test_an_engine_result_error_is_tagged_for_the_broker():
    class _Broken:
        def get_backtest_result(self, request):
            raise_response_error(common_pb.Response(task="get_result", error="a reported metric has no value"))

    context = _Context()
    with pytest.raises(_Aborted):
        SessionServiceServicer(_Broken()).get_result(None, context)  # type: ignore[arg-type]
    assert context.code == grpc.StatusCode.INTERNAL
    assert context.details == ENGINE_RESULT_ERROR_TAG + "a reported metric has no value"


def test_a_lost_engine_is_not_tagged_as_a_result_error():
    class _Gone:
        def get_backtest_result(self, request):
            raise TimeoutError("timed out")

    with pytest.raises(TimeoutError):
        SessionServiceServicer(_Gone()).get_result(None, _Context())  # type: ignore[arg-type]


def test_an_engine_error_is_an_engine_request_error():
    with pytest.raises(EngineRequestError, match="no result"):
        raise_response_error(common_pb.Response(task="get_result", error="no result"))


class _Socket:
    """Answers one request on a new context, then stops the request loop."""

    def __init__(self, request: common_pb.Request):
        self.requests = [request.to_binary()]
        self.sent: list[bytes] = []

    def new_context(self):
        return self

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False

    def recv(self) -> bytes:
        if not self.requests:
            raise StopExcecution()
        return self.requests.pop(0)

    def send(self, data: bytes):
        self.sent.append(data)


class _ReplyOnce:
    """One request/reply. A second send is the BadState that used to kill the engine."""

    def __init__(self, socket: "_Requests"):
        self.socket = socket
        self.sends = 0

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False

    def recv(self) -> bytes:
        if not self.socket.requests:
            raise StopExcecution()
        return self.socket.requests.pop(0)

    def send(self, data: bytes):
        self.sends += 1
        if self.sends > 1:
            raise pynng.BadState("already answered", _nng.NNG_ESTATE)
        self.socket.sent.append(data)


class _Requests:
    def __init__(self, *requests: common_pb.Request):
        self.requests = [request.to_binary() for request in requests]
        self.sent: list[bytes] = []

    def new_context(self):
        return _ReplyOnce(self)


def test_a_failure_after_the_run_reply_is_told_on_the_next_request():
    engine = _engine()
    cause = RuntimeError("prices missing for META on 2024-03-14")
    engine._run = lambda _data, _socket: (_Algorithm(cause), b"ready")  # type: ignore[method-assign]
    socket = _Requests(common_pb.Request(task="run"), common_pb.Request(task="get_current_period"))

    engine._process_request(None, None, socket)  # type: ignore[arg-type]
    with pytest.raises(StopExcecution):
        engine._process_request(None, None, socket)  # type: ignore[arg-type]

    replied, told = socket.sent
    started = common_pb.Response()
    merge_from_binary(started, replied)
    assert not started.has_field("error")
    assert started.data == b"ready"
    failed = common_pb.Response()
    merge_from_binary(failed, told)
    with pytest.raises(PlainSentence, match="prices missing for META on 2024-03-14") as raised:
        raise_response_error(failed)
    assert "RuntimeError" in str(raised.value)


def test_the_engine_answers_the_next_period_request_with_the_runs_failure():
    engine = _engine()
    engine._run_algorithm(_Algorithm(UnpaidDividendError("dividends of 48.500000 were not paid")))
    socket = _Socket(common_pb.Request(task="get_current_period"))
    with pytest.raises(StopExcecution):
        engine._process_request(None, None, socket)  # type: ignore[arg-type]
    (sent,) = socket.sent
    response = common_pb.Response()
    merge_from_binary(response, sent)
    with pytest.raises(PlainSentence) as raised:
        raise_response_error(response)
    assert str(raised.value) == UNPAID
