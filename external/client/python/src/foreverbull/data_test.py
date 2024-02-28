from datetime import datetime

import pandas
import pytest

from foreverbull.data import Asset, Portfolio


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


@pytest.mark.parametrize(
    "asset, has_position",
    [
        (Asset(symbol="AAPL", name="Apple Inc.", title="Apple Inc.", asset_type="EQUITY"), True),
        (Asset(symbol="MSFT", name="Microsoft Corporation", title="Microsoft Corporation", asset_type="EQUITY"), False),
        (Asset(symbol="GOOG", name="Google Inc.", title="Google Inc.", asset_type="EQUITY"), True),
    ],
)
def test_read_portfolio(database, add_portfolio, add_position, asset, has_position):
    now = datetime.now()
    portfolio_id = add_portfolio("EXC_123", now, 104.22, 23.2)
    add_position(portfolio_id, "AAPL", 100, 10.0)
    add_position(portfolio_id, "GOOG", 200, 20.0)

    with database.connect() as conn:
        portfolio = Portfolio.read("EXC_123", now, conn)
    assert portfolio is not None
    assert portfolio.cash == 104.22
    assert portfolio.value == 23.2
    assert len(portfolio.positions) == 2
    if has_position:
        assert asset in portfolio
        assert portfolio[asset].symbol == asset.symbol
        assert portfolio[asset].amount
        assert portfolio[asset].cost_basis
    else:
        assert asset not in portfolio
