from concurrent import futures
from contextlib import contextmanager

import grpc
from foreverbull.pb.backtest import engine_pb2_grpc
from foreverbull_zipline.engine import Engine, EngineProcess


class BacktestService(engine_pb2_grpc.EngineServicer):
    def __init__(self, engine: Engine):
        self.engine = engine

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
        return self.engine.stop()


@contextmanager
def grpc_server(engine: Engine, port=50055):
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    engine_pb2_grpc.add_EngineServicer_to_server(BacktestService(engine), server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    yield server
    server.stop(None)
