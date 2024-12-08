import pytest

from foreverbull_testing.data import Asset
from foreverbull_testing.data import Assets


@pytest.fixture(scope="session")
def loaded_connection(fb_database):
    engine, ensure_data = fb_database
    ensure_data(start="2023-01-01", end="2023-02-01", symbols=["AAPL", "MSFT"])
    with engine.connect() as conn:
        yield conn


def test_asset(loaded_connection):
    a = Asset(loaded_connection, "2023-01-01", "2023-02-01", "AAPL")
    assert a.symbol == "AAPL"
    assert a.stock_data is not None


def test_asset_metrics(loaded_connection):
    a = Asset(loaded_connection, "2023-01-01", "2023-02-01", "AAPL")
    a.set_metric("test", 1)
    assert a.get_metric("test") == 1
    assert a.get_metric("test2") is None
    assert a.metrics == {"test": 1}


def test_assets(loaded_connection):
    a = Assets(loaded_connection, "2023-01-01", "2023-02-01", ["AAPL", "MSFT"])
    assert a.symbols == ["AAPL", "MSFT"]
    assert a.stock_data is not None


def test_assets_metrics(loaded_connection):
    a = Assets(loaded_connection, "2023-01-01", "2023-02-01", ["AAPL", "MSFT"])
    a.set_metrics("test", {"AAPL": 1, "MSFT": 2})
    assert a.get_metrics("test").to_dict() == {"AAPL": 1, "MSFT": 2}
    assert a.get_metrics("test2").to_dict() == {}
    assert a.metrics == {"test": {"AAPL": 1, "MSFT": 2}}
