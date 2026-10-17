import time

from concurrent import futures

import grpc
import pytest

from foreverbull_zipline.engine import Engine
from foreverbull_zipline.session_service import SessionServiceServicer

from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb as engine_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb_grpc as engine_service_pb_grpc


@pytest.fixture
def uut(fb_env, fb_database, backtest_entity):
    _, verify = fb_database
    verify(backtest_entity)

    engine = Engine()
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    try:
        engine.start()
        assert engine.is_ready.wait(5.0), "engine was never ready"
        servicer = SessionServiceServicer(engine)
        engine_service_pb_grpc.EngineSessionServiceSync.add_to_server(servicer, server)
        server.add_insecure_port("[::]:60066")
        server.start()
        time.sleep(1)
        yield engine_service_pb_grpc.EngineSessionClientSync(grpc.insecure_channel("localhost:60066"))
    finally:
        server.stop(None)
        engine.stop()
        engine.join()


def test_session_service_servicer(execution, uut: engine_service_pb_grpc.EngineSessionClientSync):
    rsp = uut.run_backtest(
        engine_service_pb.RunBacktestRequest(
            execution=execution,
        )
    )
    assert rsp

    while True:
        response = uut.get_current_period(engine_service_pb.GetCurrentPeriodRequest())
        if response.is_running is False:
            break
        assert response.portfolio
        uut.place_orders_and_continue(engine_service_pb.PlaceOrdersAndContinueRequest())
