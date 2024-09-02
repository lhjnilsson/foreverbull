from foreverbull.pb.backtest import backtest_pb2, engine_pb2_grpc


class BacktestService(engine_pb2_grpc.BacktestEngineServicer):
    def __init__(self):
        self.ingestion = None
        self.backtest = None
        self.orders = []
        self.periods = []

    def Ingest(self, request, context):
        self.ingestion = request.ingestion
        return backtest_pb2.IngestResponse(ingestion=self.ingestion)

    def Configure(self, request, context):
        self.backtest = request.backtest
        return backtest_pb2.ConfigureResponse(backtest=self.backtest)

    def Run(self, request, context):
        return backtest_pb2.RunResponse(error="")

    def Continue(self, request, context):
        self.orders = request.orders
        return backtest_pb2.ContinueResponse(
            period=self.periods.pop(0),
            orders=self.orders,
            error="",
        )


from concurrent import futures

import grpc


def serve():
    server = grpc.server(thread_pool=futures.ThreadPoolExecutor())
    engine_pb2_grpc.add_EngineServicer_to_server(BacktestService(), server)
    server.add_insecure_port("[::]:50051")
    server.start()
    server.wait_for_termination()
