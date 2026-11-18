import pytest

from foreverbull_testing.database import yahoo_asset


class _Ticker:
    def __init__(self, info):
        self.info = info


def _patch_ticker(monkeypatch, info):
    seen = []

    def ticker(symbol):
        seen.append(symbol)
        return _Ticker(info)

    monkeypatch.setattr("foreverbull_testing.database.yfinance.Ticker", ticker)
    return seen


def test_yahoo_asset_returns_the_symbol_and_leaves_the_name_empty(monkeypatch):
    seen = _patch_ticker(
        monkeypatch,
        {"symbol": "AAPL", "shortName": "Apple Inc.", "quoteType": "EQUITY"},
    )

    asset = yahoo_asset("AAPL")

    assert seen == ["AAPL"]
    assert asset.symbol == "AAPL"
    assert asset.name == ""
    assert asset.ISIN == ""


def test_yahoo_asset_keeps_an_empty_symbol(monkeypatch):
    _patch_ticker(monkeypatch, {"symbol": "", "shortName": "not listed"})

    asset = yahoo_asset("NOPE")

    assert asset.symbol == ""
    assert asset.name == ""
    assert asset.ISIN == ""


def test_yahoo_asset_not_found_when_yahoo_returns_no_symbol(monkeypatch):
    _patch_ticker(monkeypatch, {})

    with pytest.raises(KeyError, match="symbol"):
        yahoo_asset("ZZZZ")
