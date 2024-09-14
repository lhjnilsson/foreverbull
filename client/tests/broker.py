import os
from typing import Generator

import pynng
from foreverbull import entity
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, broker_pb2, broker_pb2_grpc, engine_pb2, engine_pb2_grpc
from foreverbull.pb.service import service_pb2, worker_pb2


class Broker(broker_pb2_grpc.BrokerServicer):
    def __init__(self, engine: engine_pb2_grpc.EngineStub):
        self.engine = engine
        self._broker_port = 7878
        self._namespace_port = 7879
        self._worker_socket: pynng.Socket | None = None
        self._namespace_socket: pynng.Socket | None = None
        self._execution: entity.backtest.Execution | None = None

    def GetBacktest(self, request: broker_pb2.GetBacktestRequest, context) -> broker_pb2.GetBacktestResponse:
        return broker_pb2.GetBacktestResponse()

    def CreateSession(self, request: broker_pb2.CreateSessionRequest, context) -> broker_pb2.CreateSessionResponse:
        print("SESSION CREATED", flush=True)
        return broker_pb2.CreateSessionResponse(
            session=backtest_pb2.Session(
                id="abc123",
                port=6067,
            )
        )

    @staticmethod
    def namespace_server(port: int):
        pass

    def CreateExecution(
        self, request: broker_pb2.CreateExecutionRequest, context
    ) -> broker_pb2.CreateExecutionResponse:
        self._worker_socket = pynng.Req0(listen="tcp://0.0.0.0:7878")
        self._namespace_socket = pynng.Req0(listen="tcp://0.0.0.0:7879")
        self._execution = entity.backtest.Execution(
            start=pb_utils.from_proto_timestamp(request.backtest.start_date),
            end=pb_utils.from_proto_timestamp(request.backtest.end_date),
            symbols=[s for s in request.backtest.symbols],
            benchmark=request.benchmark,
        )

        return broker_pb2.CreateExecutionResponse(
            configuration=service_pb2.ExecutionConfiguration(
                brokerPort=self._broker_port,
                namespacePort=self._namespace_port,
                databaseURL=os.getenv("DATABASE_URL"),
                functions=[],
            )
        )

    def RunExecution(
        self, request: broker_pb2.RunExecutionRequest, context
    ) -> Generator[broker_pb2.RunExecutionResponse, None, None]:
        if self._worker_socket is None:
            raise Exception("Session not created")

        rsp = self.engine.RunBacktest(
            engine_pb2.RunRequest(
                backtest=backtest_pb2.Backtest(
                    start_date=pb_utils.to_proto_timestamp(self._execution.start),
                    end_date=pb_utils.to_proto_timestamp(self._execution.end),
                    symbols=self._execution.symbols,
                    benchmark=self._execution.benchmark,
                )
            )
        )
        while True:
            rsp: engine_pb2.GetCurrentPeriodResponse = self.engine.GetCurrentPeriod(
                engine_pb2.GetCurrentPeriodRequest()
            )
            if rsp.is_running is False:
                break
            orders = []
            for symbol in self._execution.symbols:
                worker_request = worker_pb2.WorkerRequest(
                    task="handle_data",
                    symbols=[symbol],
                    portfolio=rsp.portfolio,
                )
                self._worker_socket.send(worker_request.SerializeToString())
                worker_response = worker_pb2.WorkerResponse()
                worker_response.ParseFromString(self._worker_socket.recv())
                orders.extend(worker_response.orders)
            self.engine.PlaceOrdersAndContinue(engine_pb2.PlaceOrdersAndContinueRequest(orders=orders))
            yield broker_pb2.RunExecutionResponse(
                portfolio=rsp.portfolio,
            )

    def GetExecution(self, request: broker_pb2.GetExecutionRequest, context) -> broker_pb2.GetExecutionResponse:
        rsp: engine_pb2.GetResultResponse = self.engine.GetResult(engine_pb2.GetResultRequest())
        return broker_pb2.GetExecutionResponse(
            periods=rsp.periods,
        )
