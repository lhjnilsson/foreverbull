from functools import wraps
from typing import Callable
from typing import Concatenate

import grpc

from foreverbull.pb.foreverbull.finance.service.v1 import trading_pb2_grpc as trading_service_pb2_grpc


def trading_servicer[R, **P](
    f: Callable[Concatenate[trading_service_pb2_grpc.TradingServicerStub, P], R],
):
    servicer = trading_service_pb2_grpc.TradingServicerStub(grpc.insecure_channel("localhost:50055"))

    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        return f(servicer, *args, **kwargs)

    return wrapper
