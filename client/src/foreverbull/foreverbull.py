from collections.abc import Callable
from collections.abc import Generator
from contextlib import contextmanager
from multiprocessing import Event

import grpc
import pandas

from foreverbull import models
from foreverbull.broker.config import broker_host as default_broker_host
from foreverbull.broker.config import resolve_broker_address
from foreverbull.broker.config import split_broker_address
from foreverbull.exceptions import RuntimeError
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
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2
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
    ) -> Generator[tuple[str, session_service_pb2.RunExecutionResponse], None, None]:
        with WorkerPool(self._algorithm._file_path) as wp:
            req = session_service_pb2.CreateExecutionRequest(
                session=self._session.id,
                start_date=start,
                end_date=end,
                algorithm=self._algorithm.get_definition(),
            )
            create: session_service_pb2.CreateExecutionResponse = self._stub.CreateExecution(req)
            try:
                wp.configure_execution(create.configuration)
                wp.run_execution(Event())
                rsp = self._stub.RunExecution(session_service_pb2.RunExecutionRequest(execution_id=create.execution.id))
                for message in rsp:
                    yield create.execution.id, message

            except Exception as e:
                raise RuntimeError("Error running backtest") from e
            self._stub.StoreResult(
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
        symbols: list[str],
    ) -> tuple[list[marketdata_pb2.Order], Callable]:
        with WorkerPool(self._model._file_path) as wp:
            try:
                req = trading_service_pb2.ConfigureRequest(
                    algorithm=self._model.get_definition(),
                )
                rsp = self._stub.Configure(req)
                wp.configure_execution(rsp.configuration)
                wp.run_execution(Event())
                req = trading_service_pb2.RunRequest(
                    symbols=symbols,
                )
                rsp = self._stub.Run(req)
            except Exception as e:
                raise RuntimeError("Error running trading session") from e

            def place_orders(orders: list[marketdata_pb2.Order]):
                req = trading_service_pb2.PlaceOrderRequest(
                    orders=orders,
                )
                rsp = self._stub.PlaceOrders(req)
                return rsp

            return rsp.orders, place_orders


class Foreverbull:
    def __init__(self, model: models.Algorithm | None):
        self._model = model

    @contextmanager
    def backtest_session(
        self,
        backtest: backtest_pb2.Backtest,
        backtest_session: session_pb2.Session,
        broker_hostname: str | None = None,
    ) -> Generator[BacktestSession, None, None]:
        hostname = default_broker_host() if broker_hostname is None else broker_hostname
        channel = grpc.insecure_channel(f"{hostname}:{backtest_session.port}")
        _backtest_session_stub = session_service_pb2_grpc.SessionServicerStub(channel)
        assert self._model is not None, "no model"
        yield BacktestSession(self._model, backtest, backtest_session, _backtest_session_stub)
        _backtest_session_stub.StopServer(session_service_pb2.StopServerRequest())
        channel.close()

    @contextmanager
    def trading_session(
        self,
        trading_session: trading_pb2.Session,
        broker_hostname: str | None = None,
    ) -> Generator[TradingSession, None, None]:
        assert self._model is not None, "no model"
        assert trading_session is not None, "session is not set"
        hostname = default_broker_host() if broker_hostname is None else broker_hostname
        channel = grpc.insecure_channel(f"{hostname}:{trading_session.port}")
        trading_session_stub = trading_service_pb2_grpc.SessionServicerStub(channel)
        yield TradingSession(self._model, trading_session_stub)
        trading_session_stub.StopServer(session_service_pb2.StopServerRequest())
        channel.close()

    def get_execution(
        self,
        execution_id: str,
        broker_hostname: str | None = None,
        broker_port: int | None = None,
    ) -> tuple[execution_pb2.Execution, pandas.DataFrame]:
        if broker_hostname is None or broker_port is None:
            host, port = split_broker_address(resolve_broker_address())
            hostname = host if broker_hostname is None else broker_hostname
            port = port if broker_port is None else broker_port
        else:
            hostname = broker_hostname
            port = broker_port
        channel = grpc.insecure_channel(f"{hostname}:{port}")
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
