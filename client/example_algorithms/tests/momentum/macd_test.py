from datetime import datetime

import pytest

from example_algorithms.momentum.macd import handle_data
from foreverbull import Portfolio
from foreverbull.pb.foreverbull.finance import finance_pb2
from foreverbull_testing.data import Assets


@pytest.fixture(scope="session")
def assets(fb_database):
    engine, ensure_data = fb_database
    ensure_data()
    with engine.connect() as conn:
        assets = Assets(conn, datetime(2021, 1, 1), datetime(2021, 3, 31), symbols=["AAPL", "MSFT", "GOOGL"])
        return assets


@pytest.fixture(scope="function")
def portfolio(fb_database):
    engine, _ = fb_database
    with engine.connect() as conn:
        p = Portfolio(finance_pb2.Portfolio(), conn)
        yield p


def test_handle_data(assets, portfolio):
    res = handle_data(assets, portfolio)
    print("RES: ", res)
