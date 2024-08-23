from foreverbull_testing.data import Asset, Assets


def test_asset():
    a = Asset("2020-01-01", "2021-02-01", "AAPL")
    assert a.symbol == "AAPL"
    assert a.stock_data is not None


def test_asset_metrics():
    a = Asset("2020-01-01", "2021-02-01", "AAPL")
    a.set_metric("test", 1)
    assert a.get_metric("test") == 1
    assert a.get_metric("test2") is None
    assert a.metrics == {"test": 1}


def test_assets():
    a = Assets("2020-01-01", "2021-02-01", ["AAPL", "MSFT"])
    assert a.symbols == ["AAPL", "MSFT"]
    assert a.stock_data is not None
    assert len(a.stock_data) == 2


def test_assets_metrics():
    a = Assets("2020-01-01", "2021-02-01", ["AAPL", "MSFT"])
    a.set_metrics("test", {"AAPL": 1, "MSFT": 2})
    assert a.get_metrics("test") == {"AAPL": 1, "MSFT": 2}
    assert a.get_metrics("test2") == {}
    assert a.metrics == {"test": {"AAPL": 1, "MSFT": 2}}
