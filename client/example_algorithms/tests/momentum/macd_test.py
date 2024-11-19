from datetime import datetime

import pytest

from example_algorithms.momentum.macd import handle_data
from foreverbull_testing.data import DateLimitedAssets


@pytest.fixture(scope="session")
def assets():
    assets = DateLimitedAssets(start=datetime(2021, 1, 1), end=datetime(2021, 3, 31), symbols=["AAPL", "MSFT", "GOOGL"])
    return assets


def test_handle_data(assets):
    res = handle_data(assets, None)  # type: ignore

    print("RES: ", res)
