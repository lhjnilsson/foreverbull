from datetime import datetime

import pandas
import pytest

from foreverbull.data import Asset


def test_asset_stock_data(database, ingest_config):
    with database.connect() as conn:
        for symbol in ingest_config.symbols:
            a = Asset.read(symbol, datetime.now(), conn)
            assert a is not None
            assert a.symbol == symbol
            assert a.name is not None
            assert a.title is not None
            assert a.asset_type == "EQUITY"
            stock_data = a.stock_data
            assert stock_data is not None
            assert isinstance(stock_data, pandas.DataFrame)
            assert len(stock_data) > 0
            assert "open" in stock_data.columns
            assert "high" in stock_data.columns
            assert "low" in stock_data.columns
            assert "close" in stock_data.columns
            assert "volume" in stock_data.columns
