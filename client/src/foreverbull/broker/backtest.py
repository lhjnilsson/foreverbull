from collections.abc import Callable
from collections.abc import Generator
from functools import wraps
from typing import Concatenate

import grpc

from protobuf import Oneof

from foreverbull.broker.config import resolve_broker_address
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb as backtest_service_pb
from foreverbull.pb.foreverbull.backtest.service.v1 import backtest_pb_grpc as backtest_service_pb_grpc
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb
from foreverbull.pb.foreverbull.backtest.v1 import execution_pb
from foreverbull.pb.foreverbull.backtest.v1 import session_pb
from foreverbull.pb.pb_utils import required


def backtest_servicer[R, **P](
    f: Callable[Concatenate[backtest_service_pb_grpc.BacktestServicerClientSync, P], R],
) -> Callable[P, R]:
    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        servicer = backtest_service_pb_grpc.BacktestServicerClientSync(grpc.insecure_channel(resolve_broker_address()))
        return f(servicer, *args, **kwargs)

    return wrapper


@backtest_servicer
def list_backtests(
    servicer: backtest_service_pb_grpc.BacktestServicerClientSync,
) -> list[backtest_pb.Backtest]:
    return servicer.list_backtests(backtest_service_pb.ListBacktestsRequest()).backtests


@backtest_servicer
def create(
    servicer: backtest_service_pb_grpc.BacktestServicerClientSync,
    entity: backtest_service_pb.CreateBacktestRequest.CreateFromETF
    | backtest_service_pb.CreateBacktestRequest.CreateFromBacktest,
) -> Generator[backtest_service_pb.CreateBacktestResponse, None, None]:
    match entity:
        case backtest_service_pb.CreateBacktestRequest.CreateFromETF():
            req = backtest_service_pb.CreateBacktestRequest(create=Oneof(field="etf", value=entity))
        case backtest_service_pb.CreateBacktestRequest.CreateFromBacktest():
            req = backtest_service_pb.CreateBacktestRequest(create=Oneof(field="backtest", value=entity))
        case _:
            raise ValueError("Invalid entity")
    yield from servicer.create_backtest(req)


@backtest_servicer
def get(servicer: backtest_service_pb_grpc.BacktestServicerClientSync, name: str) -> backtest_pb.Backtest:
    req = backtest_service_pb.GetBacktestRequest(
        name=name,
    )
    rsp = servicer.get_backtest(req)
    return required(rsp.backtest, "backtest")


@backtest_servicer
def delete(servicer: backtest_service_pb_grpc.BacktestServicerClientSync, name: str) -> None:
    req = backtest_service_pb.DeleteBacktestRequest(
        name=name,
    )
    servicer.delete_backtest(req)


@backtest_servicer
def create_session(
    servicer: backtest_service_pb_grpc.BacktestServicerClientSync, backtest_name: str
) -> Generator[backtest_service_pb.CreateSessionResponse, None, None]:
    req = backtest_service_pb.CreateSessionRequest(
        backtest_name=backtest_name,
    )
    yield from servicer.create_session(req)


@backtest_servicer
def get_session(servicer: backtest_service_pb_grpc.BacktestServicerClientSync, session_id: str) -> session_pb.Session:
    req = backtest_service_pb.GetSessionRequest(
        session_id=session_id,
    )
    rsp: backtest_service_pb.GetSessionResponse = servicer.get_session(req)
    return required(rsp.session, "session")


@backtest_servicer
def list_executions(
    servicer: backtest_service_pb_grpc.BacktestServicerClientSync,
    backtest: str | None = None,
    session_id: str | None = None,
) -> list[execution_pb.Execution]:
    return servicer.list_executions(
        backtest_service_pb.ListExecutionsRequest(backtest=backtest or "", session_id=session_id or "")
    ).executions


@backtest_servicer
def get_execution(
    servicer: backtest_service_pb_grpc.BacktestServicerClientSync, execution_id: str
) -> tuple[execution_pb.Execution, list[execution_pb.Period]]:
    req = backtest_service_pb.GetExecutionRequest(
        execution_id=execution_id,
    )
    rsp = servicer.get_execution(req)
    return required(rsp.execution, "execution"), rsp.periods


@backtest_servicer
def get_execution_response(
    servicer: backtest_service_pb_grpc.BacktestServicerClientSync, execution_id: str
) -> backtest_service_pb.GetExecutionResponse:
    """The whole response, including the run-level universe fields `get_execution` drops."""
    rsp = servicer.get_execution(backtest_service_pb.GetExecutionRequest(execution_id=execution_id))
    required(rsp.execution, "execution")
    return rsp
