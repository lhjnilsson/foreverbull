import minio

from foreverbull.pb.foreverbull.backtest import engine_service_pb2
from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc


def test_ingest(engine_stub: engine_service_pb2_grpc.EngineStub, storage: minio.Minio):
    engine_stub.GetCurrentPeriod(engine_service_pb2.GetCurrentPeriodRequest())


def test_download_ingestion():
    pass


def test_run_with_bad_ingestion():
    pass
