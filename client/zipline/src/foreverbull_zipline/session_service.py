from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb_grpc as engine_service_pb_grpc
from foreverbull_zipline.engine import Engine


class SessionServiceServicer(engine_service_pb_grpc.EngineSessionServiceSync):
    def __init__(self, engine: Engine):
        self.engine = engine

    def run_backtest(self, request, context):
        return self.engine.run_backtest(request)

    def get_current_period(self, request, context):
        return self.engine.get_current_period(request)

    def place_orders_and_continue(self, request, context):
        return self.engine.place_orders_and_continue(request)

    def get_result(self, request, context):
        return self.engine.get_backtest_result(request)
