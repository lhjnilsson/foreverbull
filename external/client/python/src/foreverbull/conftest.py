from datetime import timedelta

import pytest

from foreverbull import worker
from foreverbull.entity.finance import Portfolio
from foreverbull.entity.service import Request, Response


@pytest.fixture(scope="session")
def process_symbols(ingest_config):
    def _process_symbols(server_socket, parallel):
        start = ingest_config.start
        portfolio = Portfolio(cash=0, value=0, positions=[])
        while start < ingest_config.end:
            if parallel:
                for symbol in ingest_config.symbols:
                    req = worker.Request(timestamp=start, symbols=[symbol], portfolio=portfolio)
                    server_socket.send(Request(task="", data=req).dump())
                    response = Response.load(server_socket.recv())
                    assert response.task == ""
                    assert response.error is None
            else:
                req = worker.Request(timestamp=start, symbols=ingest_config.symbols, portfolio=portfolio)
                server_socket.send(Request(task="", data=req).dump())
                response = Response.load(server_socket.recv())
                assert response.task == ""
                assert response.error is None
            start += timedelta(days=1)

    return _process_symbols
