import os

from collections.abc import Generator

import pynng

from protobuf import merge_from_binary

from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb as engine_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import engine_pb_grpc as engine_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb as session_service_pb
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.foreverbull.backtest.v1 import session_pb
from foreverbull.pb.foreverbull.service.service.v1 import worker_pb as worker_service_pb
from foreverbull.pb.foreverbull.service.v1 import worker_pb


class Broker:
    def __init__(self, engine: engine_service_pb_grpc.EngineSessionClientSync):
        self.engine = engine
        self._broker_port = 7878
        self._namespace_port = 7879
        self._worker_socket: pynng.Socket | None = None
        self._namespace_socket: pynng.Socket | None = None
        self._backtest: backtest_pb.Backtest | None = None

    def list_backtests(
        self, request: backtest_service_pb.ListBacktestsRequest, context
    ) -> backtest_service_pb.ListBacktestsResponse:
        return backtest_service_pb.ListBacktestsResponse()

    def create_backtest(
        self, request: backtest_service_pb.CreateBacktestRequest, context
    ) -> backtest_service_pb.CreateBacktestResponse:
        return backtest_service_pb.CreateBacktestResponse()

    def get_backtest(
        self, request: backtest_service_pb.GetBacktestRequest, context
    ) -> backtest_service_pb.GetBacktestResponse:
        return backtest_service_pb.GetBacktestResponse()

    def create_session(
        self, request: backtest_service_pb.CreateSessionRequest, context
    ) -> backtest_service_pb.CreateSessionResponse:
        return backtest_service_pb.CreateSessionResponse(
            session=session_pb.Session(
                id="abc123",
                port=6067,
            )
        )

    def get_session(
        self, request: backtest_service_pb.GetSessionRequest, context
    ) -> backtest_service_pb.GetSessionResponse:
        return backtest_service_pb.GetSessionResponse(
            session=session_pb.Session(
                id="abc123",
                port=6067,
            )
        )

    def list_executions(
        self, request: backtest_service_pb.ListExecutionsRequest, context
    ) -> backtest_service_pb.ListExecutionsResponse:
        return backtest_service_pb.ListExecutionsResponse()

    def get_execution(
        self, request: backtest_service_pb.GetExecutionRequest, context
    ) -> backtest_service_pb.GetExecutionResponse:
        rsp: engine_service_pb.GetResultResponse = self.engine.get_result(engine_service_pb.GetResultRequest())
        return backtest_service_pb.GetExecutionResponse(
            periods=rsp.periods,
        )

    @staticmethod
    def namespace_server(port: int):
        pass

    def create_execution(
        self, request: session_service_pb.CreateExecutionRequest, context
    ) -> session_service_pb.CreateExecutionResponse:
        self._worker_socket = pynng.Req0(listen="tcp://0.0.0.0:7878")
        self._namespace_socket = pynng.Req0(listen="tcp://0.0.0.0:7879")
        self._execution = request

        return session_service_pb.CreateExecutionResponse(
            execution=execution_pb.Execution(),
            configuration=worker_pb.ExecutionConfiguration(
                brokerPort=self._broker_port,
                namespacePort=self._namespace_port,
                databaseURL=os.getenv("DATABASE_URL") or "",
                functions=[],
            ),
        )

    def run_execution(
        self, request: session_service_pb.RunExecutionRequest, context
    ) -> Generator[session_service_pb.RunExecutionResponse, None, None]:
        if self._worker_socket is None or self._execution is None:
            raise RuntimeError("Session not created")

        self.engine.run_backtest(engine_service_pb.RunBacktestRequest())
        while True:
            rsp = self.engine.get_current_period(engine_service_pb.GetCurrentPeriodRequest())
            if rsp.is_running is False:
                break
            orders = []
            if self._backtest is None:
                raise RuntimeError("Backtest not set")
            symbols = [a.symbol for period in self._backtest.periods for a in period.assets if a.enabled]
            for symbol in symbols:
                worker_request = worker_service_pb.WorkerRequest(
                    task="handle_data",
                    symbols=[symbol],
                    portfolio=rsp.portfolio,
                )
                self._worker_socket.send(worker_request.to_binary())
                worker_response = worker_service_pb.WorkerResponse()
                merge_from_binary(worker_response, self._worker_socket.recv())
                orders.extend(worker_response.orders)
            self.engine.place_orders_and_continue(engine_service_pb.PlaceOrdersAndContinueRequest(orders=orders))
            yield session_service_pb.RunExecutionResponse(
                portfolio=rsp.portfolio,
            )

    def store_result(
        self, request: session_service_pb.StoreExecutionResultRequest, context
    ) -> session_service_pb.StoreExecutionResultResponse:
        return session_service_pb.StoreExecutionResultResponse()

    def stop_server(
        self, request: session_service_pb.StopServerRequest, context
    ) -> session_service_pb.StopServerResponse:
        return session_service_pb.StopServerResponse()
