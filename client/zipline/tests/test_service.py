import sys
import time

from concurrent import futures
from datetime import date

import grpc
import pytest

from foreverbull_zipline import service

from foreverbull import exceptions
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb as engine_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb_grpc as engine_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.finance.service.v1 import calendar_pb as calendar_service_pb
from foreverbull.pb.foreverbull.finance.service.v1 import calendar_pb_grpc as calendar_service_pb_grpc


class TestCalendarService:
    @pytest.fixture
    def servicer(self, fb_env):
        server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
        servicer = service.CalendarService()
        calendar_service_pb_grpc.CalendarServiceSync.add_to_server(servicer, server)
        server.add_insecure_port("[::]:60066")
        server.start()
        time.sleep(1)
        try:
            yield server
        finally:
            server.stop(None)

    @pytest.fixture
    def stub(self, servicer):
        return calendar_service_pb_grpc.CalendarClientSync(grpc.insecure_channel("localhost:60066"))

    def test_list_calendar_dates(self, stub: calendar_service_pb_grpc.CalendarClientSync):
        req = calendar_service_pb.ListCalendarDatesRequest(
            code="XNYS",
            start=pb_utils.from_pydate_to_proto_date(date(2020, 1, 1)),
            end=pb_utils.from_pydate_to_proto_date(date(2020, 12, 31)),
        )
        response = stub.list_calendar_dates(req)
        assert response


class TestBacktestService:
    @pytest.fixture
    def servicer(self):
        server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
        servicer = service.BacktestService()
        engine_service_pb_grpc.EngineServiceSync.add_to_server(servicer, server)
        server.add_insecure_port("[::]:60066")
        server.start()
        time.sleep(1)
        try:
            yield server
        finally:
            servicer.stop()
            server.stop(None)

    @pytest.fixture
    def stub(self, servicer):
        return engine_service_pb_grpc.EngineClientSync(grpc.insecure_channel("localhost:60066"))

    def test_ingest(
        self,
        stub: engine_service_pb_grpc.EngineClientSync,
        backtest_entity: backtest_pb.Backtest,
        fb_database,
    ):
        _, verify = fb_database
        verify(backtest_entity)
        response = stub.ingest(
            engine_service_pb.IngestRequest(
                backtest=backtest_entity,
            )
        )
        assert response

    def test_ingest_missing_driver_reports_the_driver_without_the_password(
        self,
        stub: engine_service_pb_grpc.EngineClientSync,
        monkeypatch,
        caplog,
    ):
        secret = "n0t-the-default-Pa55"
        monkeypatch.setenv("DATABASE_URL", f"postgresql+psycopg://foreverbull:{secret}@127.0.0.1:1/foreverbull")
        # sys.modules[...] = None makes the import fail the way a missing package does.
        monkeypatch.setitem(sys.modules, "psycopg", None)

        with pytest.raises(grpc.RpcError) as raised:
            stub.ingest(engine_service_pb.IngestRequest(backtest=backtest_pb.Backtest(name="missing-driver")))

        assert raised.value.code() == grpc.StatusCode.FAILED_PRECONDITION
        message = raised.value.details() or ""
        reason, _, details = message.partition(service.INGEST_DETAILS_MARKER)
        assert reason == "the data service is missing its database driver (psycopg)"
        assert details.startswith("ModuleNotFoundError:")
        assert secret not in message
        assert secret not in caplog.text
        assert "Could not connect" not in message

    def test_new_session(
        self,
        stub: engine_service_pb_grpc.EngineClientSync,
        fb_env,
    ):
        response = stub.new_session(engine_service_pb.NewSessionRequest(id="test123"))
        assert response.port


class TestIngestFailure:
    def test_missing_driver(self):
        exc = exceptions.DatabaseDriverError(
            "Database driver psycopg is not installed",
            driver="psycopg",
            cause="ModuleNotFoundError: No module named 'psycopg'",
        )
        code, details = service.ingest_failure(exc)
        assert code == grpc.StatusCode.FAILED_PRECONDITION
        assert details == (
            "the data service is missing its database driver (psycopg)"
            "\nDetails: ModuleNotFoundError: No module named 'psycopg'"
        )

    def test_connection_error_is_still_a_connection_error(self):
        exc = exceptions.ConnectionError(
            "Could not connect to database at postgresql://foreverbull:***@db.internal:5432/foreverbull",
            cause="OperationalError: connection refused",
        )
        code, details = service.ingest_failure(exc)
        assert code == grpc.StatusCode.UNAVAILABLE
        assert details == (
            "the data service could not connect to database at "
            "postgresql://foreverbull:***@db.internal:5432/foreverbull"
            "\nDetails: OperationalError: connection refused"
        )

    def test_other_error_is_redacted(self):
        code, details = service.ingest_failure(ValueError("bad bundle at postgresql://u:hunter22@h/db"))
        assert code == grpc.StatusCode.INTERNAL
        assert details == (
            "the data service failed while ingesting prices\nDetails: ValueError: bad bundle at postgresql://u:***@h/db"
        )
