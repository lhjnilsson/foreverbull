from functools import wraps
from typing import Callable
from typing import Concatenate

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
def get_portfolio(stub: trading_service_pb2_grpc.TradingServicerStub) -> trading_service_pb2.GetPortfolioResponse:
    return stub.GetPortfolio(trading_service_pb2.GetPortfolioRequest())


@trading_servicer
def get_orders(stub: trading_service_pb2_grpc.TradingServicerStub) -> trading_service_pb2.GetOrdersResponse:
    return stub.GetOrders(trading_service_pb2.GetOrdersRequest())
