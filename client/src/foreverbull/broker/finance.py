from collections.abc import Callable
from collections.abc import Generator
from functools import wraps
from typing import Concatenate

from foreverbull.broker.config import resolve_broker_address
from foreverbull.broker.credentials import broker_channel
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb as trading_service_pb
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb_grpc as trading_service_pb_grpc


def trading_servicer[R, **P](
    f: Callable[Concatenate[trading_service_pb_grpc.TradingServicerClientSync, P], R],
):
    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        servicer = trading_service_pb_grpc.TradingServicerClientSync(broker_channel(resolve_broker_address()))
        return f(servicer, *args, **kwargs)

    return wrapper


@trading_servicer
def get_portfolio(
    stub: trading_service_pb_grpc.TradingServicerClientSync,
) -> trading_service_pb.GetPortfolioResponse:
    return stub.get_portfolio(trading_service_pb.GetPortfolioRequest())


@trading_servicer
def get_orders(
    stub: trading_service_pb_grpc.TradingServicerClientSync,
) -> trading_service_pb.GetOrdersResponse:
    return stub.get_orders(trading_service_pb.GetOrdersRequest())


@trading_servicer
def create_session(
    stub: trading_service_pb_grpc.TradingServicerClientSync, symbols: list[str]
) -> Generator[trading_service_pb.CreateSessionResponse, None, None]:
    rsp = stub.create_session(trading_service_pb.CreateSessionRequest(symbols=symbols))
    yield from rsp
