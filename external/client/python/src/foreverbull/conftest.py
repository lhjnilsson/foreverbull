import os
import tempfile
from datetime import timedelta

import pytest

from foreverbull import entity, worker
from foreverbull.entity.finance import Order, Portfolio
from foreverbull.entity.service import Request, Response


@pytest.fixture(scope="session")
def process_symbols(
    ingest_config,
):
    def _process_symbols(
        server_socket,
        execution,
    ):
        start = ingest_config.start
        portfolio = Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        while start < ingest_config.end:
            for symbol in ingest_config.symbols:
                req = worker.Request(
                    execution=execution,
                    timestamp=start,
                    symbol=symbol,
                    portfolio=portfolio,
                )
                server_socket.send(
                    Request(
                        task="",
                        data=req,
                    ).dump()
                )
                response = Response.load(server_socket.recv())
                assert response.task == ""
                assert response.error is None
                if response.data:
                    order = Order(**response.data)
                    assert order.symbol == symbol
            start += timedelta(days=1)

    return _process_symbols


@pytest.fixture(scope="session")
def execution(
    database,
):
    return entity.service.Execution(
        id="test",
        port=5656,
        database_url=os.environ.get(
            "DATABASE_URL",
            "",
        ),
        configuration={
            "handle_data": entity.service.Execution.Function(
                parameters={
                    "low": "5",
                    "high": "10",
                }
            )
        },
    )


@pytest.fixture(scope="session")
def parallel_algo(
    ingest_config,
    database,
):
    def _process_symbols(
        server_socket,
        execution,
    ):
        start = ingest_config.start
        portfolio = Portfolio(
            cash=0,
            value=0,
            positions=[],
        )
        while start < ingest_config.end:
            for symbol in ingest_config.symbols:
                req = worker.Request(
                    execution=execution,
                    timestamp=start,
                    symbol=symbol,
                    portfolio=portfolio,
                )
                server_socket.send(
                    Request(
                        task="handle_data",
                        data=req,
                    ).dump()
                )
                response = Response.load(server_socket.recv())
                assert response.task == "handle_data"
                assert response.error is None
                if response.data:
                    order = Order(**response.data)
                    assert order.symbol == symbol
            start += timedelta(days=1)

    e = entity.service.Execution(
        id="test",
        port=5656,
        database_url=os.environ.get(
            "DATABASE_URL",
            "",
        ),
        configuration={
            "handle_data": entity.service.Execution.Function(
                parameters={
                    "low": "5",
                    "high": "10",
                }
            )
        },
    )

    with tempfile.NamedTemporaryFile(suffix=".py") as f:
        f.write(
            b"""
from foreverbull import Algorithm, Function, Portfolio, Order, Asset

def handle_data(asses: Asset, portfolio: Portfolio, low: int = 5, high: int = 10) -> Order:
    pass
    
Algorithm(
    functions=[
        Function(callable=handle_data)
    ]
)
"""
        )
        f.flush()
        yield f.name, e, _process_symbols
