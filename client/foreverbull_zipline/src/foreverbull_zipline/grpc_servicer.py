from concurrent import futures
from contextlib import contextmanager

import grpc
from foreverbull.pb import health_pb2, health_pb2_grpc
from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc
from foreverbull_zipline.engine import Engine


class BacktestService(engine_service_pb2_grpc.EngineServicer):
    def __init__(self, engine: Engine):
        self.engine = engine

    def DownloadIngestion(self, request, context):
        return self.engine.download_ingestion(request)

    def Ingest(self, request, context):
        return self.engine.ingest(request)

    def RunBacktest(self, request, context):
        return self.engine.run_backtest(request)

    def GetCurrentPeriod(self, request, context):
        return self.engine.get_current_period(request)

    def PlaceOrdersAndContinue(self, request, context):
        return self.engine.place_orders_and_continue(request)

    def GetResult(self, request, context):
        return self.engine.get_backtest_result(request)

    def Stop(self, request, context):
        return self.engine.stop(request)


class BacktestServiceWithHealthCheck(BacktestService, health_pb2_grpc.HealthServicer):
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(status=health_pb2.HealthCheckResponse.SERVING)


@contextmanager
def grpc_server(engine: Engine, port=50055):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    bs = BacktestServiceWithHealthCheck(engine)
    engine_service_pb2_grpc.add_EngineServicer_to_server(bs, server)
    health_pb2_grpc.add_HealthServicer_to_server(bs, server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    yield server
    server.stop(None)
