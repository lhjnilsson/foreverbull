import time
from contextlib import contextmanager
from datetime import datetime
from functools import wraps
from multiprocessing import Event
from typing import Generator

import grpc
from foreverbull import entity, models
from foreverbull.pb import pb_utils
from foreverbull.pb.backtest import backtest_pb2, broker_pb2, broker_pb2_grpc
from foreverbull.pb.finance import finance_pb2  # noqa
from foreverbull.pb.service import service_pb2
from foreverbull.worker import WorkerPool


class Algorithm(models.Algorithm):
    def __init__(self, functions: list[models.Function], namespaces: list[str] = []):
        self._broker = None
        self._backtest_session = None
        super().__init__(functions, namespaces)

    @contextmanager
    def backtest_session(self, backtest_name: str, broker_hostname: str = "localhost", broker_port: int = 50051):
        channel = grpc.insecure_channel(f"{broker_hostname}:{broker_port}")
        self._broker = broker_pb2_grpc.BrokerStub(channel)
        self._backtest_session: backtest_pb2.Session | None = None

        session = self._broker.CreateSession(broker_pb2.CreateSessionRequest(backtest_name=backtest_name))
        while session.port is None:
            session = self._broker.GetSession(broker_pb2.GetSessionRequest(session_id=session.id))
            if session.statuses[0].status == entity.backtest.SessionStatusType.FAILED:
                raise Exception(f"Session failed: {session.statuses[-1].error}")
            time.sleep(0.5)
        self._backtest_session = session.session
        yield self
        channel.close()

    @staticmethod
    def _has_session(func):
        @wraps(func)
        def wrapper(self, *args, **kwargs):
            if self._backtest_session is None:
                raise RuntimeError("No backtest session")
            return func(self, *args, **kwargs)

        return wrapper

    @_has_session
    def get_default(self) -> entity.backtest.Backtest:
        backtest: broker_pb2.GetBacktestResponse = self._broker.GetBacktest(
            broker_pb2.GetBacktestRequest(name=self._backtest_session.backtest)
        )
        return entity.backtest.Backtest(
            name=backtest.name,
            start=backtest.start,
            end=backtest.end,
            benchmark=backtest.benchmark,
            symbols=backtest.symbols,
        )

    @_has_session
    def run_execution(
        self, start: datetime, end: datetime, symbols: list[str], benchmark=None
    ) -> Generator[entity.backtest.Portfolio, None, None]:
        with WorkerPool(self._file_path) as wp:
            req = broker_pb2.CreateExecutionRequest(
                session_id=self._backtest_session.id,
                backtest=backtest_pb2.Backtest(
                    start_date=pb_utils.to_proto_timestamp(start),
                    end_date=pb_utils.to_proto_timestamp(end),
                    symbols=symbols,
                    benchmark=benchmark,
                ),
            )
            rsp = self._broker.CreateExecution(req)
            configure_req = service_pb2.ConfigureExecutionRequest()
            wp.configure_execution(configure_req)
            wp.run_execution(service_pb2.RunExecutionRequest(), Event())
            rsp = self._broker.RunExecution(broker_pb2.RunExecutionRequest(execution_id=self._backtest_session.id))
            for message in rsp:
                yield message.portfolio

        response = self._broker.CreateExecution(broker_pb2.CreateExecutionRequest(session_id=self._backtest_session.id))
        for message in response:
            yield message.portfolio
