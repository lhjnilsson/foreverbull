from collections.abc import Callable
from collections.abc import Generator
from contextlib import contextmanager
from multiprocessing import Event

import grpc
import pandas

from foreverbull import models
from foreverbull.broker.config import broker_host_port
from foreverbull.broker.credentials import broker_channel
from foreverbull.broker.credentials import session_channel
from foreverbull.exceptions import RuntimeError
from foreverbull.pb.foreverbull import common_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb_grpc as backtest_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb as session_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import session_pb_grpc as session_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.foreverbull.backtest.v1 import session_pb
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb as trading_service_pb
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb_grpc as trading_service_pb_grpc
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb
from foreverbull.pb.foreverbull.finance.v1 import trading_pb
from foreverbull.pb.pb_utils import required
from foreverbull.worker import WorkerPool


class BacktestSession:
    def __init__(
        self,
        algorithm: models.Algorithm,
        backtest: backtest_pb.Backtest,
        session: session_pb.Session,
        stub: session_service_pb_grpc.SessionServicerClientSync,
    ):
        self._algorithm = algorithm
        self._backtest = backtest
        self._session = session
        self._stub = stub

    @property
    def algorithm(self) -> models.Algorithm:
        return self._algorithm

    @property
    def backtest(self) -> backtest_pb.Backtest:
        return self._backtest

    @property
    def session(self) -> session_pb.Session:
        return self._session

    def run_execution(
        self,
        start: common_pb.Date,
        end: common_pb.Date,
    ) -> Generator[tuple[str, session_service_pb.RunExecutionResponse], None, None]:
        with WorkerPool(self._algorithm._file_path) as wp:
            req = session_service_pb.CreateExecutionRequest(
                session=self._session.id,
                start_date=start,
                end_date=end,
                algorithm=self._algorithm.get_definition(),
            )
            create: session_service_pb.CreateExecutionResponse = self._stub.create_execution(req)
            try:
                configuration = required(create.configuration, "execution configuration")
                execution = required(create.execution, "execution")
                wp.configure_execution(configuration)
                wp.run_execution(Event())
                rsp = self._stub.run_execution(session_service_pb.RunExecutionRequest(execution_id=execution.id))
                for message in rsp:
                    yield execution.id, message

            except grpc.RpcError:
                raise
            except Exception as e:
                raise RuntimeError("Error running backtest") from e
            self._stub.store_result(
                session_service_pb.StoreExecutionResultRequest(
                    execution_id=execution.id,
                )
            )


class TradingSession:
    def __init__(
        self,
        model: models.Algorithm,
        stub: trading_service_pb_grpc.SessionServicerClientSync,
    ):
        self._model = model
        self._stub = stub

    def run_trading(
        self,
        symbols: list[str],
    ) -> tuple[list[marketdata_pb.Order], Callable]:
        with WorkerPool(self._model._file_path) as wp:
            try:
                req = trading_service_pb.ConfigureRequest(
                    algorithm=self._model.get_definition(),
                )
                rsp = self._stub.configure(req)
                wp.configure_execution(required(rsp.configuration, "execution configuration"))
                wp.run_execution(Event())
                req = trading_service_pb.RunRequest(
                    symbols=symbols,
                )
                rsp = self._stub.run(req)
            except grpc.RpcError:
                raise
            except Exception as e:
                raise RuntimeError("Error running trading session") from e

            def place_orders(orders: list[marketdata_pb.Order]):
                req = trading_service_pb.PlaceOrderRequest(
                    orders=orders,
                )
                rsp = self._stub.place_orders(req)
                return rsp

            return rsp.orders, place_orders


class Foreverbull:
    def __init__(self, model: models.Algorithm | None):
        self._model = model

    @contextmanager
    def backtest_session(
        self,
        backtest: backtest_pb.Backtest,
        backtest_session: session_pb.Session,
        broker_hostname: str | None = None,
        session_secret: str = "",
    ) -> Generator[BacktestSession, None, None]:
        hostname, _ = broker_host_port(broker_hostname)
        channel = session_channel(f"{hostname}:{backtest_session.port}", session_secret)
        _backtest_session_stub = session_service_pb_grpc.SessionServicerClientSync(channel)
        assert self._model is not None, "no model"
        yield BacktestSession(self._model, backtest, backtest_session, _backtest_session_stub)
        _backtest_session_stub.stop_server(session_service_pb.StopServerRequest())
        channel.close()

    @contextmanager
    def trading_session(
        self,
        trading_session: trading_pb.Session,
        broker_hostname: str | None = None,
        session_secret: str = "",
    ) -> Generator[TradingSession, None, None]:
        assert self._model is not None, "no model"
        assert trading_session is not None, "session is not set"
        hostname, _ = broker_host_port(broker_hostname)
        channel = session_channel(f"{hostname}:{trading_session.port}", session_secret)
        trading_session_stub = trading_service_pb_grpc.SessionServicerClientSync(channel)
        yield TradingSession(self._model, trading_session_stub)
        trading_session_stub.stop_server(trading_service_pb.StopServerRequest())
        channel.close()

    def get_execution(
        self,
        execution_id: str,
        broker_hostname: str | None = None,
        broker_port: int | None = None,
    ) -> tuple[execution_pb.Execution, pandas.DataFrame]:
        hostname, port = broker_host_port(broker_hostname, broker_port)
        channel = broker_channel(f"{hostname}:{port}")
        stub = backtest_service_pb_grpc.BacktestServicerClientSync(channel)

        rsp = stub.get_execution(backtest_service_pb.GetExecutionRequest(execution_id=execution_id))
        periods = []
        for period in rsp.periods:
            periods.append(
                {
                    "portfolio_value": period.portfolio_value,
                    "returns": period.returns,
                    "alpha": period.alpha if period.has_field("alpha") else None,
                    "beta": period.beta if period.has_field("beta") else None,
                    "sharpe": period.sharpe if period.has_field("sharpe") else None,
                }
            )
        return required(rsp.execution, "execution"), pandas.DataFrame(periods).reset_index(drop=True)
