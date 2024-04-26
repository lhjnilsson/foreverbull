from datetime import datetime

import pandas

from foreverbull.data import Asset, Assets


def test_asset(database, ingest_config):
    with database.connect() as conn:
        for symbol in ingest_config.symbols:
            asset = Asset(datetime.now(), conn, symbol)
            assert asset is not None
            assert asset.symbol == symbol
            stock_data = asset.stock_data
            assert stock_data is not None
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) > 0
            assert "open" in stock_data.columns
            assert "high" in stock_data.columns
            assert "low" in stock_data.columns
            assert "close" in stock_data.columns
            assert "volume" in stock_data.columns


def test_assets(database, ingest_config):
    with database.connect() as conn:
        assets = Assets(datetime.now(), conn, ingest_config.symbols)
        for asset in assets:
            assert asset is not None
            assert asset.symbol is not None
            stock_data = asset.stock_data
            assert stock_data is not None
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) > 0
            assert "open" in stock_data.columns
            assert "high" in stock_data.columns
            assert "low" in stock_data.columns
            assert "close" in stock_data.columns
            assert "volume" in stock_data.columns
