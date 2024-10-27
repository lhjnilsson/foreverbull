import os

from functools import wraps
from typing import Callable
from typing import Concatenate

import grpc

from foreverbull.pb.foreverbull.finance import finance_service_pb2
from foreverbull.pb.foreverbull.finance import finance_service_pb2_grpc


def finance_servicer[R, **P](
    f: Callable[Concatenate[finance_service_pb2_grpc.FinanceStub, P], R],
) -> Callable[P, R]:
    port = os.getenv("BROKER_PORT", "50055")
    servicer = finance_service_pb2_grpc.FinanceStub(grpc.insecure_channel(f"localhost:{port}"))

    @wraps(f)
    def wrapper(*args: P.args, **kwargs: P.kwargs):
        return f(servicer, *args, **kwargs)

    return wrapper


@finance_servicer
def get_index(
    servicer: finance_service_pb2_grpc.FinanceStub,
    symbol: str,
) -> finance_service_pb2.GetIndexResponse:
    req = finance_service_pb2.GetIndexRequest(
        symbol=symbol,
    )
    return servicer.GetIndex(req)


@finance_servicer
def download_historical_data(
    servicer: finance_service_pb2_grpc.FinanceStub,
    req: finance_service_pb2.DownloadHistoricalDataRequest,
) -> finance_service_pb2.DownloadHistoricalDataResponse:
    return servicer.DownloadHistoricalData(req)
