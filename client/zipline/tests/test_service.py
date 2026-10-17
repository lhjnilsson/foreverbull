import time

from concurrent import futures
from datetime import date

import grpc
import pytest

from foreverbull_zipline import service

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

    def test_new_session(
        self,
        stub: engine_service_pb_grpc.EngineClientSync,
        fb_env,
    ):
        response = stub.new_session(engine_service_pb.NewSessionRequest(id="test123"))
        assert response.port
