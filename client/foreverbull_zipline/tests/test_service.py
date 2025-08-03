import time

from concurrent import futures
from datetime import date

import grpc
import pytest

from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2 as engine_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2_grpc as engine_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import calendar_pb2 as calendar_service_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import calendar_pb2_grpc as calendar_service_pb2_grpc
from foreverbull_zipline import service


class TestCalendarService:
    @pytest.fixture
    def servicer(self):
        server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
        servicer = service.CalendarService()
        calendar_service_pb2_grpc.add_CalendarServicer_to_server(servicer, server)
        server.add_insecure_port("[::]:60066")
        server.start()
        time.sleep(1)
        try:
            yield server
        finally:
            server.stop(None)

    @pytest.fixture
    def stub(self, servicer):
        return calendar_service_pb2_grpc.CalendarStub(grpc.insecure_channel("localhost:60066"))

    def test_list_calendar_dates(self, stub: calendar_service_pb2_grpc.CalendarStub):
        req = calendar_service_pb2.ListCalendarDatesRequest(
            code="XNYS",
            start=pb_utils.from_pydate_to_proto_date(date(2020, 1, 1)),
            end=pb_utils.from_pydate_to_proto_date(date(2020, 12, 31)),
        )
        response = stub.ListCalendarDates(req)
        assert response


class TestBacktestService:
    @pytest.fixture
    def servicer(self, backtest_entity, fb_database):
        _, verify = fb_database
        verify(backtest_entity)

        server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
        servicer = service.BacktestService()
        engine_service_pb2_grpc.add_EngineServicer_to_server(servicer, server)
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
        return engine_service_pb2_grpc.EngineStub(grpc.insecure_channel("localhost:60066"))

    def test_ingest(
        self,
        stub: engine_service_pb2_grpc.EngineStub,
        backtest_entity: backtest_pb2.Backtest,
    ):
        response = stub.Ingest(
            engine_service_pb2.IngestRequest(
                backtest=backtest_entity,
            )
        )
        assert response

    def test_new_session(
        self,
        stub: engine_service_pb2_grpc.EngineStub,
    ):
        response = stub.NewSession(engine_service_pb2.NewSessionRequest(id="test123"))
        assert response.port
