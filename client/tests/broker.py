import os

from collections.abc import Generator

import pynng

from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2 as engine_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb2_grpc as engine_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2 as session_service_pb2
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb2 as worker_service_pb2
from foreverbull.pb.foreverbull.service.v1 import worker_pb2


class Broker:
    def __init__(self, engine: engine_service_pb2_grpc.EngineSessionStub):
        self.engine = engine
        self._broker_port = 7878
        self._namespace_port = 7879
        self._worker_socket: pynng.Socket | None = None
        self._namespace_socket: pynng.Socket | None = None
        self._backtest: backtest_pb2.Backtest | None = None

    def ListBacktests(
        self, request: backtest_service_pb2.ListBacktestsRequest, context
    ) -> backtest_service_pb2.ListBacktestsResponse:
        return backtest_service_pb2.ListBacktestsResponse()

    def CreateBacktest(
        self, request: backtest_service_pb2.CreateBacktestRequest, context
    ) -> backtest_service_pb2.CreateBacktestResponse:
        return backtest_service_pb2.CreateBacktestResponse()

    def GetBacktest(
        self, request: backtest_service_pb2.GetBacktestRequest, context
    ) -> backtest_service_pb2.GetBacktestResponse:
        return backtest_service_pb2.GetBacktestResponse()

    def CreateSession(
        self, request: backtest_service_pb2.CreateSessionRequest, context
    ) -> backtest_service_pb2.CreateSessionResponse:
        return backtest_service_pb2.CreateSessionResponse(
            session=session_pb2.Session(
                id="abc123",
                port=6067,
            )
        )

    def GetSession(
        self, request: backtest_service_pb2.GetSessionRequest, context
    ) -> backtest_service_pb2.GetSessionResponse:
        return backtest_service_pb2.GetSessionResponse(
            session=session_pb2.Session(
                id="abc123",
                port=6067,
            )
        )

    def ListExecutions(
        self, request: backtest_service_pb2.ListExecutionsRequest, context
    ) -> backtest_service_pb2.ListExecutionsResponse:
        return backtest_service_pb2.ListExecutionsResponse()

    def GetExecution(
        self, request: backtest_service_pb2.GetExecutionRequest, context
    ) -> backtest_service_pb2.GetExecutionResponse:
        rsp: engine_service_pb2.GetResultResponse = self.engine.GetResult(engine_service_pb2.GetResultRequest())
        return backtest_service_pb2.GetExecutionResponse(
            periods=rsp.periods,
        )

    @staticmethod
    def namespace_server(port: int):
        pass

    def CreateExecution(
        self, request: session_service_pb2.CreateExecutionRequest, context
    ) -> session_service_pb2.CreateExecutionResponse:
        self._worker_socket = pynng.Req0(listen="tcp://0.0.0.0:7878")
        self._namespace_socket = pynng.Req0(listen="tcp://0.0.0.0:7879")
        self._execution = request

        return session_service_pb2.CreateExecutionResponse(
            configuration=worker_pb2.ExecutionConfiguration(
                brokerPort=self._broker_port,
                namespacePort=self._namespace_port,
                databaseURL=os.getenv("DATABASE_URL"),
                functions=[],
            )
        )

    def RunExecution(
        self, request: session_service_pb2.RunExecutionRequest, context
    ) -> Generator[session_service_pb2.RunExecutionResponse, None, None]:
        if self._worker_socket is None or self._execution is None:
            raise RuntimeError("Session not created")

        rsp = self.engine.RunBacktest(engine_service_pb2.RunBacktestRequest())
        while True:
            rsp: engine_service_pb2.GetCurrentPeriodResponse = self.engine.GetCurrentPeriod(
                engine_service_pb2.GetCurrentPeriodRequest()
            )
            if rsp.is_running is False:
                break
            orders = []
            if self._backtest is None:
                raise RuntimeError("Backtest not set")
            symbols = [a.symbol for period in self._backtest.periods for a in period.assets if a.enabled]
            for symbol in symbols:
                worker_request = worker_service_pb2.WorkerRequest(
                    task="handle_data",
                    symbols=[symbol],
                    portfolio=rsp.portfolio,
                )
                self._worker_socket.send(worker_request.SerializeToString())
                worker_response = worker_service_pb2.WorkerResponse()
                worker_response.ParseFromString(self._worker_socket.recv())
                orders.extend(worker_response.orders)
            self.engine.PlaceOrdersAndContinue(engine_service_pb2.PlaceOrdersAndContinueRequest(orders=orders))
            yield session_service_pb2.RunExecutionResponse(
                portfolio=rsp.portfolio,
            )

    def StoreResult(
        self, request: session_service_pb2.StoreExecutionResultRequest, context
    ) -> session_service_pb2.StoreExecutionResultResponse:
        return session_service_pb2.StoreExecutionResultResponse()

    def StopServer(
        self, request: session_service_pb2.StopServerRequest, context
    ) -> session_service_pb2.StopServerResponse:
        return session_service_pb2.StopServerResponse()
