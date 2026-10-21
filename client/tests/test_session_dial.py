from concurrent import futures
from typing import cast

import grpc

from foreverbull import models
from foreverbull.broker.credentials import _MetadataInterceptor
from foreverbull.broker.credentials import resolve_token
from foreverbull.broker.credentials import set_cli_token
from foreverbull.foreverbull import Foreverbull
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb_grpc as backtest_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb as session_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb_grpc as session_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.foreverbull.backtest.v1 import session_pb
from foreverbull.worker import WorkerDaemon
from foreverbull.worker import _run_worker_process


class _Details:
    method = "/foreverbull.example/Call"
    timeout = None
    metadata = None
    credentials = None
    wait_for_ready = None
    compression = None


def _pairs(metadata) -> dict[str, str]:
    found: dict[str, str] = {}
    for key, value in metadata or ():
        if isinstance(value, bytes):
            value = value.decode()
        found[key] = value
    return found


def test_bearer_interceptor_sets_stream_metadata():
    seen: dict[str, object] = {}

    def continuation(details, request):
        seen["metadata"] = details.metadata
        return request

    interceptor = _MetadataInterceptor((("authorization", "Bearer fb_x"),))
    assert interceptor.intercept_unary_stream(continuation, _Details(), "req") == "req"
    assert interceptor.intercept_unary_unary(continuation, _Details(), "req") == "req"
    assert _pairs(seen["metadata"])["authorization"] == "Bearer fb_x"


def test_worker_process_drops_broker_token(monkeypatch):
    monkeypatch.setenv("FOREVERBULL_TOKEN", "fb_secret")
    seen: dict[str, str | None] = {}

    class _Daemon:
        def run(self):
            seen["left"] = __import__("os").environ.get("FOREVERBULL_TOKEN")

    _run_worker_process(cast(WorkerDaemon, _Daemon()))
    assert seen["left"] is None


def _serve(servicer, add):
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    add(servicer, server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    return server, port


def test_session_dial_sets_metadata_and_omits_empty_secret(monkeypatch, tmp_path):
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("FOREVERBULL_TOKEN", "fb_user")
    set_cli_token(None)
    captured: list[dict[str, str]] = []

    def stop_server(request, context):
        captured.append(_pairs(context.invocation_metadata()))
        return session_service_pb.StopServerResponse()

    servicer = __import__("unittest.mock", fromlist=["MagicMock"]).MagicMock()
    servicer.stop_server.side_effect = stop_server
    server, port = _serve(servicer, session_service_pb_grpc.SessionServicerServiceSync.add_to_server)
    try:
        session = session_pb.Session(port=port)
        model = cast(models.Algorithm, object())
        with Foreverbull(model).backtest_session(backtest_pb.Backtest(), session, session_secret="sek") as live:
            assert live.session.port == port
        with Foreverbull(model).backtest_session(backtest_pb.Backtest(), session, session_secret="  "):
            pass
    finally:
        server.stop(None)
    assert captured[0].get("x-foreverbull-session") == "sek"
    assert "authorization" not in captured[0]
    assert "x-foreverbull-session" not in captured[1]
    assert "authorization" not in captured[1]
    assert resolve_token(environ={"FOREVERBULL_TOKEN": "fb_user"}, home=tmp_path) == "fb_user"


def test_get_execution_sends_bearer_only_when_configured(monkeypatch, tmp_path):
    monkeypatch.setenv("HOME", str(tmp_path))
    set_cli_token(None)
    captured: list[dict[str, str]] = []

    def get_execution(request, context):
        captured.append(_pairs(context.invocation_metadata()))
        return backtest_service_pb.GetExecutionResponse(execution=execution_pb.Execution(id=request.execution_id))

    servicer = __import__("unittest.mock", fromlist=["MagicMock"]).MagicMock()
    servicer.get_execution.side_effect = get_execution
    server, port = _serve(servicer, backtest_service_pb_grpc.BacktestServicerServiceSync.add_to_server)
    try:
        monkeypatch.delenv("FOREVERBULL_TOKEN", raising=False)
        Foreverbull(None).get_execution("exec-1", broker_hostname="127.0.0.1", broker_port=port)
        monkeypatch.setenv("FOREVERBULL_TOKEN", " fb_abc ")
        Foreverbull(None).get_execution("exec-1", broker_hostname="127.0.0.1", broker_port=port)
    finally:
        server.stop(None)
    assert "authorization" not in captured[0]
    assert captured[1].get("authorization") == "Bearer fb_abc"
