import time

from concurrent import futures

import grpc
import pytest

from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.session_service import SessionServiceServicer


@pytest.fixture
def uut():
    engine = Engine()
    engine.start()
    assert engine.is_ready.wait(5.0), "engine was never ready"
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    servicer = SessionServiceServicer(engine)
    engine_service_pb2_grpc.add_EngineSessionServicer_to_server(servicer, server)
    server.add_insecure_port("[::]:60066")
    server.start()
    time.sleep(1)
    yield engine_service_pb2_grpc.EngineSessionStub(grpc.insecure_channel("localhost:60066"))
    server.stop(None)
    engine.stop()
    engine.join()


def test_session_service_servicer(execution, uut: engine_service_pb2_grpc.EngineSessionStub):
    rsp = uut.RunBacktest(
        engine_service_pb2.RunBacktestRequest(
            backtest=backtest_pb2.Backtest(
                start_date=execution.start_date,
                end_date=execution.end_date,
                symbols=execution.symbols,
                benchmark=None,
            )
        )
    )
    assert rsp

    while True:
        response = uut.GetCurrentPeriod(engine_service_pb2.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        uut.PlaceOrdersAndContinue(engine_service_pb2.PlaceOrdersAndContinueRequest())
