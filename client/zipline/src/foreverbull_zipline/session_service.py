import grpc

from foreverbull.broker.utils import redact_text
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb_grpc as engine_service_pb_grpc
from foreverbull_zipline.engine import Engine
from foreverbull_zipline.errors import PlainSentence


class SessionServiceServicer(engine_service_pb_grpc.EngineSessionServiceSync):
    def __init__(self, engine: Engine):
        self.engine = engine

    def run_backtest(self, request, context):
        try:
            return self.engine.run_backtest(request)
        except PlainSentence as exc:
            # A refused run: the broker shows this sentence to `fbull` as is.
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, redact_text(str(exc)))

    def get_current_period(self, request, context):
        return self.engine.get_current_period(request)

    def place_orders_and_continue(self, request, context):
        return self.engine.place_orders_and_continue(request)

    def get_result(self, request, context):
        return self.engine.get_backtest_result(request)
