import time

from concurrent import futures

import grpc
import pytest

from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest import ingestion_pb2
from foreverbull_zipline import service


@pytest.fixture
def servicer(fb_database):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    servicer = service.BacktestService()
    engine_service_pb2_grpc.add_EngineServicer_to_server(servicer, server)
    server.add_insecure_port("[::]:60066")
    server.start()
    time.sleep(1)  # wait for server to start, abit hacky
    yield server
    servicer.stop()


@pytest.fixture
def stub(servicer):
    return engine_service_pb2_grpc.EngineStub(grpc.insecure_channel("localhost:60066"))


def test_get_ingestion(stub: engine_service_pb2_grpc.EngineStub):
    pass


def test_ingest(
    stub: engine_service_pb2_grpc.EngineStub,
    backtest_entity: backtest_pb2.Backtest,
):
    response = stub.Ingest(
        engine_service_pb2.IngestRequest(
            ingestion=ingestion_pb2.Ingestion(
                start_date=backtest_entity.start_date,
                end_date=backtest_entity.end_date,
                symbols=backtest_entity.symbols,
            )
        )
    )
    assert response


def test_new_session(
    stub: engine_service_pb2_grpc.EngineStub,
):
    response = stub.NewSession(engine_service_pb2.NewSessionRequest(id="test123"))
    assert response.port
    print("RESPONSE ", response)
