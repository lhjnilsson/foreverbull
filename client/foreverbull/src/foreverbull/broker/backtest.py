from functools import wraps
from typing import Callable
from typing import Concatenate
from typing import Generator

import grpc

from foreverbull.pb.foreverbull import common_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2 as backtest_service_pb2
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb2_grpc as backtest_service_pb2_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb2
from foreverbull.pb.foreverbull.backtest.v1 import session_pb2


def backtest_servicer[R, **P](
    f: Callable[Concatenate[backtest_service_pb2_grpc.BacktestServicerStub, P], R],
) -> Callable[P, R]:
    servicer = backtest_service_pb2_grpc.BacktestServicerStub(grpc.insecure_channel("localhost:50055"))

    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        return f(servicer, *args, **kwargs)

    return wrapper


@backtest_servicer
def list_backtests(servicer: backtest_service_pb2_grpc.BacktestServicerStub) -> list[backtest_pb2.Backtest]:
    return servicer.ListBacktests(backtest_service_pb2.ListBacktestsRequest()).backtests


@backtest_servicer
def create(
    servicer: backtest_service_pb2_grpc.BacktestServicerStub,
    name: str,
    base_etf: str,
    start_date: common_pb2.Date,
    end_date: common_pb2.Date,
) -> Generator[backtest_service_pb2.CreateBacktestResponse, None, None]:
    req = backtest_service_pb2.CreateBacktestRequest(
        name=name,
        base_etf=base_etf,
        start_date=start_date,
        end_date=end_date,
    )
    return servicer.CreateBacktest(req)


@backtest_servicer
def get(servicer: backtest_service_pb2_grpc.BacktestServicerStub, name: str) -> backtest_pb2.Backtest:
    req = backtest_service_pb2.GetBacktestRequest(
        name=name,
    )
    rsp = servicer.GetBacktest(req)
    return rsp.backtest


@backtest_servicer
def create_session(
    servicer: backtest_service_pb2_grpc.BacktestServicerStub, backtest_name: str
) -> Generator[backtest_service_pb2.CreateSessionResponse, None, None]:
    req = backtest_service_pb2.CreateSessionRequest(
        backtest_name=backtest_name,
    )
    for rsp in servicer.CreateSession(req):
        yield rsp


@backtest_servicer
def get_session(servicer: backtest_service_pb2_grpc.BacktestServicerStub, session_id: str) -> session_pb2.Session:
    req = backtest_service_pb2.GetSessionRequest(
        session_id=session_id,
    )
    rsp: backtest_service_pb2.GetSessionResponse = servicer.GetSession(req)
    return rsp.session


@backtest_servicer
def list_executions(
    servicer: backtest_service_pb2_grpc.BacktestServicerStub, backtest: str | None = None, session_id: str | None = None
) -> list[execution_pb2.Execution]:
    return servicer.ListExecutions(
        backtest_service_pb2.ListExecutionsRequest(backtest=backtest, session_id=session_id)
    ).executions


@backtest_servicer
def get_execution(
    servicer: backtest_service_pb2_grpc.BacktestServicerStub, execution_id: str
) -> tuple[execution_pb2.Execution, list[execution_pb2.Period]]:
    req = backtest_service_pb2.GetExecutionRequest(
        execution_id=execution_id,
    )
    rsp = servicer.GetExecution(req)
    return rsp.execution, rsp.periods
