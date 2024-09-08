from concurrent import futures

import grpc
from foreverbull.pb.backtest import engine_pb2_grpc
from foreverbull_zipline.engine import Engine


class BacktestService(engine_pb2_grpc.EngineServicer):
    def __init__(self, engine: Engine):
        self.engine = engine

    def Ingest(self, request, context):
        return self.engine.ingest(request)

    def RunBacktest(self, request, context):
        return self.engine.run_backtest(request)

    def PlaceOrders(self, request, context):
        return self.engine.place_orders(request)

    def GetNextPeriod(self, request, context):
        return self.engine.get_next_period(request)

    def GetResult(self, request, context):
        return self.engine.get_backtest_result(request)

    def Stop(self, request, context):
        return self.engine.stop()


def serve(engine: Engine) -> grpc.Server:
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor(max_workers=1))
    engine_pb2_grpc.add_EngineServicer_to_server(BacktestService(engine), server)
    server.add_insecure_port("[::]:50055")
    return server
    # server.start()
    # server.wait_for_termination()
    # server.stop(None)
