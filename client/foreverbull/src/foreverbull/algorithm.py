import time

from contextlib import contextmanager
from multiprocessing import Event
from typing import Generator

import grpc
import pandas

from foreverbull import models
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest import backtest_pb2
from foreverbull.pb.foreverbull.backtest import backtest_service_pb2
from foreverbull.pb.foreverbull.backtest import backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest import execution_pb2
from foreverbull.pb.foreverbull.backtest import session_pb2
from foreverbull.pb.foreverbull.backtest import session_service_pb2
from foreverbull.pb.foreverbull.backtest import session_service_pb2_grpc
from foreverbull.pb.foreverbull.finance import finance_pb2  # noqa
from foreverbull.pb.foreverbull.strategy import strategy_service_pb2
from foreverbull.pb.foreverbull.strategy import strategy_service_pb2_grpc
from foreverbull.worker import WorkerPool


class Algorithm(models.Algorithm):
    def __init__(self, functions: list[models.Function], namespaces: list[str] = []):
        self._broker_stub = None
        self._broker_session_stub = None
        self._backtest_session = None
        super().__init__(functions, namespaces)

    @classmethod
    def from_file_path(cls, file_path: str) -> "Algorithm":
        super().from_file_path(file_path)
        functions = []
        for k, v in models.Algorithm._functions.items():
            functions.append(models.Function(callable=v["callable"]))
        return cls(functions, models.Algorithm._namespaces)

    @contextmanager
    def backtest_session(
        self,
        backtest_name: str,
        broker_hostname: str = "localhost",
        broker_port: int = 50055,
    ):
        channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")
        self._broker_stub = backtest_service_pb2_grpc.BacktestServicerStub(channel)
        self._backtest_session: session_pb2.Session | None = None
        rsp = self._broker_stub.CreateSession(backtest_service_pb2.CreateSessionRequest(backtest_name=backtest_name))
        while not rsp.session.HasField("port"):
            rsp = self._broker_stub.GetSession(backtest_service_pb2.GetSessionRequest(session_id=rsp.session.id))
            if rsp.session.statuses and rsp.session.statuses[0].status == session_pb2.Session.Status.Status.FAILED:
                raise Exception(f"Session failed: {rsp.session.statuses[-1].error}")
            time.sleep(0.5)
        self._backtest_session = rsp.session
        self._broker_session_stub = session_service_pb2_grpc.SessionServicerStub(
            grpc.insecure_channel(f"{broker_hostname}:{rsp.session.port}")
        )
        yield self
        self._broker_session_stub.StopServer(session_service_pb2.StopServerRequest())
        channel.close()

    def run_strategy(
        self,
        from_date: common_pb2.Date,
        symbols: list[str],
        broker_hostname: str = "localhost",
        broker_port: int = 50055,
    ):
        with WorkerPool(self._file_path) as wp:
            channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")

            stub = strategy_service_pb2_grpc.StrategyServicerStub(channel)
            run_channel = stub.RunStrategy(
                strategy_service_pb2.RunStrategyRequest(
                    symbols=symbols,
                    start_date=from_date,
                    algorithm=self.get_definition(),
                )
            )
            msg: strategy_service_pb2.RunStrategyResponse
            for msg in run_channel:
                match msg.status.status:
                    case strategy_service_pb2.RunStrategyResponse.Status.Status.READY:
                        wp.configure_execution(msg.configuration)
                        wp.run_execution(Event())
                        print("DONE")  # TODO: Actual trading and return results
                    case strategy_service_pb2.RunStrategyResponse.Status.Status.FAILED:
                        channel.close()
                        raise Exception(f"Strategy failed: {msg.status.error}")

    def get_default(self) -> backtest_pb2.Backtest:
        if self._broker_stub is None or self._backtest_session is None:
            raise RuntimeError("No backtest session")
        rsp: backtest_service_pb2.GetBacktestResponse = self._broker_stub.GetBacktest(
            backtest_service_pb2.GetBacktestRequest(name=self._backtest_session.backtest)
        )
        return rsp.backtest

    def run_execution(
        self,
        start: common_pb2.Date,
        end: common_pb2.Date,
        symbols: list[str],
        benchmark=None,
    ) -> Generator[finance_pb2.Portfolio, None, None]:
        if self._broker_session_stub is None or self._backtest_session is None:
            raise RuntimeError("No backtest session")
        with WorkerPool(self._file_path) as wp:
            req = session_service_pb2.CreateExecutionRequest(
                backtest=backtest_pb2.Backtest(
                    start_date=start,
                    end_date=end,
                    symbols=symbols,
                    benchmark=benchmark,
                ),
                algorithm=self.get_definition(),
            )
            create: session_service_pb2.CreateExecutionResponse = self._broker_session_stub.CreateExecution(req)
            wp.configure_execution(create.configuration)
            wp.run_execution(Event())
            rsp = self._broker_session_stub.RunExecution(
                session_service_pb2.RunExecutionRequest(execution_id=create.execution.id)
            )
            for message in rsp:
                yield message.portfolio

            rsp = self._broker_session_stub.StoreResult(
                session_service_pb2.StoreExecutionResultRequest(
                    execution_id=create.execution.id,
                )
            )

    def get_execution(self, execution_id: str) -> tuple[execution_pb2.Execution, pandas.DataFrame]:
        if self._broker_stub is None:
            raise RuntimeError("No backtest session")
        rsp = self._broker_stub.GetExecution(backtest_service_pb2.GetExecutionRequest(execution_id=execution_id))
        periods = []
        for period in rsp.periods:
            periods.append(
                {
                    "portfolio_value": period.portfolio_value,
                    "returns": period.returns,
                    "alpha": period.alpha if period.HasField("alpha") else None,
                    "beta": period.beta if period.HasField("beta") else None,
                    "sharpe": period.sharpe if period.HasField("sharpe") else None,
                }
            )
        return rsp.execution, pandas.DataFrame(periods).reset_index(drop=True)
