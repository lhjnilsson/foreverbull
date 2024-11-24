from foreverbull_testing.data import Asset
from foreverbull_testing.data import Assets


def test_asset(fb_database):
    engine, _ = fb_database
    with engine.connect() as conn:
        a = Asset(conn, "2023-01-01", "2023-02-01", "AAPL")
        assert a.symbol == "AAPL"
        assert a.stock_data is not None


def test_asset_metrics(fb_database):
    engine, _ = fb_database
    with engine.connect() as conn:
        a = Asset(conn, "2023-01-01", "2023-02-01", "AAPL")
        a.set_metric("test", 1)
        assert a.get_metric("test") == 1
        assert a.get_metric("test2") is None
        assert a.metrics == {"test": 1}


def test_assets(fb_database):
    engine, _ = fb_database
    with engine.connect() as conn:
        a = Assets(conn, "2023-01-01", "2023-02-01", ["AAPL", "MSFT"])
        assert a.symbols == ["AAPL", "MSFT"]
        assert a.stock_data is not None

    print("DF: ", a.stock_data)


def test_assets_metrics(fb_database):
    engine, _ = fb_database
    with engine.connect() as conn:
        a = Assets(conn, "2023-01-01", "2023-02-01", ["AAPL", "MSFT"])
        a.set_metrics("test", {"AAPL": 1, "MSFT": 2})
        assert a.get_metrics("test") == {"AAPL": 1, "MSFT": 2}
        assert a.get_metrics("test2") == {}
        assert a.metrics == {"test": {"AAPL": 1, "MSFT": 2}}
