import datetime
import os
import time
from contextlib import contextmanager
from functools import wraps
from typing import Generator

import grpc
from foreverbull import entity, models
from foreverbull.data import pb_utils
from foreverbull.pb.backtest import backtest_pb2, broker_pb2, broker_pb2_grpc
from foreverbull.worker import WorkerPool


class Algorithm(models.Algorithm):
    def __init__(self, functions: list[models.Function], namespaces: list[str] = []):
        self._broker_hostname = os.getenv("BROKER_HOSTNAME", "localhost")
        self._broker_port = os.getenv("BROKER_PORT", 50051)
        self._channel = grpc.insecure_channel(f"{self._broker_hostname}:{self._broker_port}")
        self._broker = broker_pb2_grpc.BrokerStub(self._channel)
        self._backtest_session: backtest_pb2.Session | None = None
        super().__init__(functions, namespaces)

    @contextmanager
    def backtest_session(self, backtest_name: str):
        session = self._broker.CreateSession(broker_pb2.CreateSessionRequest(backtest_name=backtest_name))
        while session.port is None:
            session = self._broker.GetSession(broker_pb2.GetSessionRequest(session_id=session.id))
            if session.statuses[0].status == entity.backtest.SessionStatusType.FAILED:
                raise Exception(f"Session failed: {session.statuses[-1].error}")
            time.sleep(0.5)
        self._backtest_session = session.session
        yield self

    @staticmethod
    def has_session(func):
        @wraps(func)
        def wrapper(self, *args, **kwargs):
            if self._backtest_session is None:
                raise Exception("No backtest session")
            return func(self, *args, **kwargs)

        return wrapper

    @has_session
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

    @has_session
    def run_execution(
        self, start: datetime.datetime, end: datetime.datetime, symbols: list[str], benchmark=None
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
            wp.configure_execution(rsp)
            wp.run_execution()
            rsp = self._broker.RunExecution(broker_pb2.RunExecutionRequest(session_id=self._backtest_session.id))
            for message in rsp:
                yield message.portfolio

        response = self._broker.CreateExecution(broker_pb2.CreateExecutionRequest(session_id=self._backtest_session.id))
        for message in response:
            yield message.portfolio


def main():
    pool = WorkerPool("bad_file")
    with pool:
        pass
