from functools import wraps
from typing import Callable
from typing import Concatenate
from typing import Generator

import grpc

from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2 as trading_service_pb2
from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2_grpc as trading_service_pb2_grpc


def trading_servicer[R, **P](
    f: Callable[Concatenate[trading_service_pb2_grpc.TradingServicerStub, P], R],
):
    servicer = trading_service_pb2_grpc.TradingServicerStub(grpc.insecure_channel("localhost:50055"))

    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        return f(servicer, *args, **kwargs)

    return wrapper


@trading_servicer
def get_portfolio(
    stub: trading_service_pb2_grpc.TradingServicerStub,
) -> trading_service_pb2.GetPortfolioResponse:
    return stub.GetPortfolio(trading_service_pb2.GetPortfolioRequest())


@trading_servicer
def get_orders(
    stub: trading_service_pb2_grpc.TradingServicerStub,
) -> trading_service_pb2.GetOrdersResponse:
    return stub.GetOrders(trading_service_pb2.GetOrdersRequest())


@trading_servicer
def create_session(
    stub: trading_service_pb2_grpc.TradingServicerStub, symbols: list[str]
) -> Generator[trading_service_pb2.CreateSessionResponse, None, None]:
    rsp = stub.CreateSession(trading_service_pb2.CreateSessionRequest(symbols=symbols))
    for msg in rsp:
        yield msg
