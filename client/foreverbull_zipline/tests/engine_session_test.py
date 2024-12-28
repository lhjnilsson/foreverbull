import pytest
from foreverbull_zipline.engine_session import grpc_server
from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc
import grpc


class TestEngineSession:
    @pytest.fixture()
    def grpc_server(self):
        server, port = grpc_server()
        server.start()
        yield server, port
        server.stop(None)

    @pytest.fixture()
    def uut(self, grpc_server):
        server, port = grpc_server
        stub = engine_service_pb2_grpc.EngineSessionStub(grpc.insecure_channel(f"localhost:{port}"))
        return stub

    def test_start(self, uut):
        pass
