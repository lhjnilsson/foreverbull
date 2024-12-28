from concurrent import futures
from contextlib import contextmanager
import logging
import grpc

from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc

from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc, engine_service_pb2
from foreverbull_zipline.engine import Engine
import os
import tarfile

import pandas as pd

from zipline.data import bundles
from zipline.data.bundles.core import BundleData
from zipline.utils.paths import data_path
from zipline.utils.paths import data_root

from foreverbull.broker.storage import Storage
from foreverbull.pb import pb_utils
from foreverbull_zipline.data_bundles.foreverbull import DatabaseEngine
from foreverbull_zipline.data_bundles.foreverbull import SQLIngester
from dataclasses import dataclass
from foreverbull_zipline.session_service import SessionServiceServicer

@dataclass
class Session:
    id: str
    engine: Engine
    servicer: SessionServiceServicer
    server: grpc.Server


class BacktestService(engine_service_pb2_grpc.EngineServicer, health_pb2_grpc.HealthServicer):
    def __init__(self, engine: Engine):
        self.logger = logging.getLogger(__name__)
        self.engine = engine

        self.sessions: list[Session] = []

    @property
    def ingestion(self) -> tuple[list[str], pd.Timestamp, pd.Timestamp]:
        if self.bundle is None:
            raise LookupError("Bundle is not loaded")
        assets = self.bundle.asset_finder.retrieve_all(self.bundle.asset_finder.sids)
        start = assets[0].start_date.tz_localize("UTC")
        end = assets[0].end_date.tz_localize("UTC")
        return [a.symbol for a in assets], start, end

    def GetIngestion(self, request, context):
        pass

    def DownloadIngestion(self, request: engine_service_pb2.DownloadIngestionRequest, context):
        storage = Storage.from_environment()
        storage.download_object(request.bucket, request.object, "/tmp/ingestion.tar.gz")
        with tarfile.open("/tmp/ingestion.tar.gz", "r:gz") as tar:
            tar.extractall(data_root())
        bundles.register("foreverbull", SQLIngester())
        return engine_service_pb2.DownloadIngestionResponse()

    def Ingest(self, request: engine_service_pb2.IngestRequest, context):
        bundles.register("foreverbull", SQLIngester(), calendar_name="XNYS")
        SQLIngester.engine = DatabaseEngine()
        SQLIngester.from_date = pb_utils.from_proto_date_to_pydate(request.ingestion.start_date)
        SQLIngester.to_date = pb_utils.from_proto_date_to_pydate(request.ingestion.end_date)
        SQLIngester.symbols = [s for s in request.ingestion.symbols]
        bundles.ingest("foreverbull", os.environ, pd.Timestamp.utcnow(), [], True)
        self.bundle: BundleData = bundles.load("foreverbull", os.environ, None)
        self.logger.debug("ingestion completed")
        symbols, start, end = self.ingestion
        if request.HasField("bucket") and request.HasField("object"):
            self.logger.debug("Uploading ingestion to: %s/%s", request.bucket, request.object)
            with tarfile.open("/tmp/ingestion.tar.gz", "w:gz") as tar:
                tar.add(data_path(["foreverbull"]), arcname="foreverbull")
            storage = Storage.from_environment()
            storage.upload_object(request.bucket, request.object, "/tmp/ingestion.tar.gz")
            self.logger.debug("Ingestion uploaded")
        return engine_service_pb2.IngestResponse()

    def NewSession(self, request: engine_service_pb2.NewSessionRequest, context):
        engine = Engine(
            socket_file_path=f"/tmp/fb_{request.id}.sock",
        )

        server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
        servicer = SessionServiceServicer(engine)
        engine_service_pb2_grpc.add_EngineSessionServicer_to_server(servicer, server)
        port = server.add_insecure_port("[::]:0")
        server.start()
        self.sessions.append(Session(request.id, servicer.engine, servicer, server))
        return engine_service_pb2.NewSessionResponse(port=port)

    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(status=health_pb2.HealthCheckResponse.SERVING)

    def Watch(self, request, context):
        return self.Check(request, context)

    def stop(self):
        for session in self.sessions:
            session.server.stop(None)
            session.engine.stop()
            session.engine.join()


@contextmanager
def grpc_server(engine: Engine, port=50055):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    bs = BacktestService(engine)
    engine_service_pb2_grpc.add_EngineServicer_to_server(bs, server)
    health_pb2_grpc.add_HealthServicer_to_server(bs, server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    yield server
    server.stop(None)
