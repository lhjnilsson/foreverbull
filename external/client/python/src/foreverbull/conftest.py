from datetime import timedelta

import pytest

from foreverbull import worker
from foreverbull.entity.finance import Order
from foreverbull.entity.service import Request, Response


@pytest.fixture(scope="session")
def process_symbols(ingest_config, add_portfolio):
    def _process_symbols(server_socket, execution):
        start = ingest_config.start
        while start < ingest_config.end:
            add_portfolio(execution, start, 100000, 0)
            for symbol in ingest_config.symbols:
                req = worker.Request(execution=execution, timestamp=start, symbol=symbol)
                server_socket.send(Request(task="", data=req).dump())
                response = Response.load(server_socket.recv())
                assert response.task == ""
                assert response.error is None
                if response.data:
                    order = Order(**response.data)
                    assert order.symbol == symbol
            start += timedelta(days=1)

    return _process_symbols
