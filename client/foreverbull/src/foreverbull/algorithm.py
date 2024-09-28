import time
from contextlib import contextmanager
from datetime import datetime
from functools import wraps
from multiprocessing import Event
from typing import Generator

import grpc
import pandas
from foreverbull import entity, models
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest import (
    backtest_pb2,
    backtest_service_pb2,
    backtest_service_pb2_grpc,
    session_pb2,
    session_service_pb2,
    session_service_pb2_grpc,
)
from foreverbull.pb.foreverbull.finance import finance_pb2  # noqa
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
        broker_port: int = 50052,
    ):
        channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")
        self._broker_stub = backtest_service_pb2_grpc.BacktestServicerStub(channel)
        self._backtest_session: session_pb2.Session | None = None
        rsp = self._broker_stub.CreateSession(
            backtest_service_pb2.CreateSessionRequest(backtest_name=backtest_name)
        )
        while not rsp.session.HasField("port"):
            rsp = self._broker_stub.GetSession(
                backtest_service_pb2.GetSessionRequest(session_id=rsp.session.id)
            )
            if (
                rsp.session.statuses
                and rsp.session.statuses[0].status
                == entity.backtest.SessionStatusType.FAILED
            ):
                raise Exception(f"Session failed: {rsp.session.statuses[-1].error}")
            time.sleep(0.5)
        self._backtest_session = rsp.session
        self._broker_session_stub = session_service_pb2_grpc.SessionServicerStub(
            grpc.insecure_channel(f"{broker_hostname}:{rsp.session.port}")
        )
        yield self
        channel.close()

    @staticmethod
    def _has_session(func):
        @wraps(func)
        def wrapper(self, *args, **kwargs):
            if self._backtest_session is None or self._broker_session_stub is None:
                raise RuntimeError("No backtest session")
            return func(self, *args, **kwargs)

        return wrapper

    @_has_session
    def get_default(self) -> entity.backtest.Backtest:
        rsp: backtest_service_pb2.GetBacktestResponse = self._broker_stub.GetBacktest(
            backtest_service_pb2.GetBacktestRequest(
                name=self._backtest_session.backtest
            )
        )
        return entity.backtest.Backtest(
            name=rsp.name,
            start=pb_utils.from_proto_timestamp(rsp.backtest.start_date),
            end=pb_utils.from_proto_timestamp(rsp.backtest.end_date),
            benchmark=rsp.backtest.benchmark,
            symbols=[s for s in rsp.backtest.symbols],
        )

    @_has_session
    def run_execution(
        self, start: datetime, end: datetime, symbols: list[str], benchmark=None
    ) -> Generator[entity.finance.Portfolio, None, None]:
        with WorkerPool(self._file_path) as wp:
            req = session_service_pb2.CreateExecutionRequest(
                backtest=backtest_pb2.Backtest(
                    start_date=pb_utils.to_proto_timestamp(start),
                    end_date=pb_utils.to_proto_timestamp(end),
                    symbols=symbols,
                    benchmark=benchmark,
                ),
            )
            rsp = self._broker_session_stub.CreateExecution(req)
            wp.configure_execution(rsp.configuration)
            wp.run_execution(Event())
            rsp = self._broker_session_stub.RunExecution(
                broker_pb2.RunExecutionRequest(execution_id="123")
            )
            for message in rsp:
                yield message.portfolio

    @_has_session
    def get_execution(
        self, execution_id: str
    ) -> tuple[entity.backtest.Execution, pandas.DataFrame]:
        rsp = self._broker_session_stub.GetExecution(
            broker_pb2.GetExecutionRequest(execution_id=execution_id)
        )
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
        return entity.backtest.Execution(
            id=rsp.execution.id,
            start=pb_utils.from_proto_timestamp(rsp.execution.start_date),
            end=pb_utils.from_proto_timestamp(rsp.execution.end_date),
            symbols=[s for s in rsp.execution.symbols],
            benchmark=rsp.execution.benchmark,
        ), pandas.DataFrame(periods).reset_index(drop=True)
