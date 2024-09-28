import minio
import pytest
from foreverbull.pb.foreverbull.backtest import (
    engine_service_pb2,
    engine_service_pb2_grpc,
)


def test_ingest(engine_stub: engine_service_pb2_grpc.EngineStub, storage: minio.Minio):
    rsp = engine_stub.GetCurrentPeriod(engine_service_pb2.GetCurrentPeriodRequest())


def test_download_ingestion():
    pass


def test_run_with_bad_ingestion():
    pass
