from foreverbull_testing.data import Asset


def test_asset():
    a = Asset("2020-01-01", "2021-02-01", "AAPL")
    assert a.symbol == "AAPL"
    assert a.stock_data is not None
