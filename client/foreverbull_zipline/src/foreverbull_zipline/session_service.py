from foreverbull.pb.foreverbull.backtest import engine_service_pb2_grpc
from foreverbull_zipline.engine import Engine


class SessionServiceServicer(engine_service_pb2_grpc.EngineSessionServicer):
    def __init__(self, engine: Engine):
        self.engine = engine

    def RunBacktest(self, request, context):
        return self.engine.run_backtest(request)

    def GetCurrentPeriod(self, request, context):
        return self.engine.get_current_period(request)

    def PlaceOrdersAndContinue(self, request, context):
        return self.engine.place_orders_and_continue(request)

    def GetResult(self, request, context):
        return self.engine.get_backtest_result(request)
