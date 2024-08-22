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
        self._stock_data = t

    def get_metric[T: (int, float, bool, str)](self, key: str) -> T:
        raise NotImplementedError()

    def set_metric[T: (int, float, bool, str)](self, key: str, value: T) -> None:
        raise NotImplementedError()

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

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> dict[str, T]:
            raise NotImplementedError()

    def set_metrics[T: (int, float, bool, str)](self, key: str, value: dict[str, T]) -> None:
            raise NotImplementedError()

    @property
    def symbols(self) -> list[str]:
        return self._symbols

    def __iter__(self):
        for symbol in self._symbols:
            yield Asset(self._start, self._end, symbol)
