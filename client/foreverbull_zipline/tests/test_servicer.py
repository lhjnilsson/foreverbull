import grpc
import pytest
from foreverbull.pb.backtest import engine_pb2, engine_pb2_grpc


@pytest.fixture
def stub():
    channel = grpc.insecure_channel("localhost:50051")
    yield engine_pb2_grpc.EngineStub(channel)
    channel.close()


def test_ingest(stub):
    pass
