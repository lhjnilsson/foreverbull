from datetime import datetime
from typing import Any

import yfinance as yf
from foreverbull import interfaces
from pandas import DataFrame


class Asset(interfaces.Asset):
    def __init__(self, start: str | datetime, end: str | datetime, symbol: str):
        self._start = start
        self._end = end
        self._symbol = symbol
        ticker = yf.Ticker(symbol)
        t = ticker.history(start=start, end=end, interval="1d")
        t.drop(columns=["Dividends", "Stock Splits"], inplace=True)
        t.rename(columns={"High": "high", "Low": "low", "Open": "open", "Close": "close", "Volume": "volume"}, inplace=True)
        t.index.name = "date"
        self._stock_data = t
        self.metrics = {}

    def get_metric[T: (int, float, bool, str)](self, key: str) -> T:
        try:
            return self.metrics[key]
        except KeyError:
            return None

    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        self.metrics[key] = value

    @property
    def symbol(self) -> str:
        return self._symbol

    @property
    def stock_data(self) -> DataFrame:
        return self._stock_data

class Assets(interfaces.Assets):
    def __init__(self, start: str, end: str, symbols: list[str]):
        self._start = start
        self._end = end
        self._symbols = symbols
        self.metrics = {}

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> dict[str, T]:
        try:
            return self.metrics[key]
        except KeyError:
            return {}

    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
        self.metrics[key] = value

    @property
    def symbols(self) -> list[str]:
        return self._symbols

    def __iter__(self):
        for symbol in self._symbols:
            yield Asset(self._start, self._end, symbol)

    @property
    def stock_data(self) -> DataFrame:
        data = {"symbols": [], "stock_data": []}
        for asset in self:
            data["symbols"].append(asset.symbol)
            data["stock_data"].append(asset.stock_data)
        return data
