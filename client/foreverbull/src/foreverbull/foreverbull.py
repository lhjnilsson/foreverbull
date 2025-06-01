from contextlib import contextmanager
from multiprocessing import Event
from typing import Generator

import grpc
import pandas

from foreverbull import models
from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2_grpc as backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2 as session_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb2_grpc as session_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2 as trading_service_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2_grpc as trading_service_pb2_grpc
from foreverbull.pb.foreverbull.finance.v1 import trading_pb2
from foreverbull.worker import WorkerPool


class BacktestSession:
    def __init__(
        self,
        algorithm: models.Algorithm,
        backtest: backtest_pb2.Backtest,
        session: session_pb2.Session,
        stub: session_service_pb2_grpc.SessionServicerStub,
    ):
        self._algorithm = algorithm
        self._backtest = backtest
        self._session = session
        self._stub = stub

    @property
    def algorithm(self) -> models.Algorithm:
        return self._algorithm

    @property
    def backtest(self) -> backtest_pb2.Backtest:
        return self._backtest

    @property
    def session(self) -> session_pb2.Session:
        return self._session

    def run_execution(
        self,
        start: common_pb2.Date,
        end: common_pb2.Date,
        symbols: list[str],
        benchmark=None,
    ) -> Generator[tuple[str, session_service_pb2.RunExecutionResponse], None, None]:
        with WorkerPool(self._algorithm._file_path) as wp:
            req = session_service_pb2.CreateExecutionRequest(
                backtest=backtest_pb2.Backtest(
                    start_date=start,
                    end_date=end,
                    symbols=symbols,
                    benchmark=benchmark,
                ),
                algorithm=self._algorithm.get_definition(),
            )
            create: session_service_pb2.CreateExecutionResponse = self._stub.CreateExecution(req)
            wp.configure_execution(create.configuration)
            wp.run_execution(Event())
            rsp = self._stub.RunExecution(session_service_pb2.RunExecutionRequest(execution_id=create.execution.id))
            for message in rsp:
                yield create.execution.id, message

            rsp = self._stub.StoreResult(
                session_service_pb2.StoreExecutionResultRequest(
                    execution_id=create.execution.id,
                )
            )


class TradingSession:
    def __init__(
        self,
        model: models.Algorithm,
        stub: trading_service_pb2_grpc.SessionServicerStub,
    ):
        self._model = model
        self._stub = stub

    def run_trading(
        self,
    ):
        with WorkerPool(self._model._file_path) as wp:
            req = trading_service_pb2.ConfigureRequest(
                algorithm=self._model.get_definition(),
            )
            rsp = self._stub.Configure(req)
            wp.configure_execution(rsp.configuration)
            wp.run_execution(Event())
            req = trading_service_pb2.RunRequest()
            rsp = self._stub.Run(req)
            return rsp.orders


class Foreverbull:
    def __init__(self, model: models.Algorithm | None):
        self._model = model

    @contextmanager
    def backtest_session(
        self,
        backtest_name: str,
        broker_hostname: str = "localhost",
        broker_port: int = 50055,
    ) -> Generator[BacktestSession, None, None]:
        channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")
        self._broker_stub = backtest_service_pb2_grpc.BacktestServicerStub(channel)

        backtest = self._broker_stub.GetBacktest(backtest_service_pb2.GetBacktestRequest(name=backtest_name))
        session: session_pb2.Session | None = None

        create_rsp = self._broker_stub.CreateSession(
            backtest_service_pb2.CreateSessionRequest(backtest_name=backtest_name)
        )
        rsp = backtest_service_pb2.CreateSessionResponse()
        for rsp in create_rsp:
            if rsp.status.status == session_pb2.SessionStatus.FAILED:
                raise Exception(f"Session failed: {rsp.status.status}")
            if rsp.status.status == session_pb2.SessionStatus.RUNNING:
                session = rsp.session
                break
        _backtest_session_stub = session_service_pb2_grpc.SessionServicerStub(
            grpc.insecure_channel(f"{broker_hostname}:{rsp.session.port}")
        )
        assert self._model is not None, "no model"
        assert session is not None, "session is not set"
        yield BacktestSession(self._model, backtest, session, _backtest_session_stub)
        _backtest_session_stub.StopServer(session_service_pb2.StopServerRequest())
        channel.close()

    @contextmanager
    def trading_session(
        self,
        symbols: list[str],
        broker_hostname: str = "localhost",
        broker_port: int = 50055,
    ) -> Generator[TradingSession, None, None]:
        channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")
        self._broker_stub = trading_service_pb2_grpc.TradingServicerStub(channel)
        trading_session: trading_pb2.Session | None = None

        create_rsp = self._broker_stub.CreateSession(trading_service_pb2.CreateSessionRequest(symbols=symbols))
        rsp: trading_service_pb2.CreateSessionResponse
        for rsp in create_rsp:
            if rsp.status.status == trading_service_pb2.CreateSessionResponse.SessionStatus.FAILED:
                raise Exception(f"Session failed: {rsp.status.status}")
            if rsp.status.status == trading_service_pb2.CreateSessionResponse.SessionStatus.RUNNING:
                trading_session = rsp.session
                break
        assert self._model is not None, "no model"
        assert trading_session is not None, "session is not set"
        trading_session_stub = trading_service_pb2_grpc.SessionServicerStub(
            grpc.insecure_channel(f"{broker_hostname}:{trading_session.port}")
        )
        yield TradingSession(self._model, trading_session_stub)
        trading_session_stub.StopServer(session_service_pb2.StopServerRequest())
        channel.close()

    def get_execution(
        self, execution_id: str, broker_hostname: str = "localhost", broker_port: int = 50055
    ) -> tuple[execution_pb2.Execution, pandas.DataFrame]:
        channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")
        stub = backtest_service_pb2_grpc.BacktestServicerStub(channel)

        rsp = stub.GetExecution(backtest_service_pb2.GetExecutionRequest(execution_id=execution_id))
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
