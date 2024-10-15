from contextlib import contextmanager
from datetime import datetime
from typing import Any, Generator

import yfinance as yf
from pandas import DataFrame
from typing import Union
from foreverbull import Asset, Assets  # type: ignore


class DateLimitedAsset(Asset):
    def __init__(self, symbol: str, df: DataFrame):
        self._symbol = symbol
        self._stock_data = df
        self.metrics = {}

    def get_metric[T: (int, float, bool, str)](self, key: str) -> Union[T, None]:
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


class Asset(Asset):
    def __init__(self, start: str | datetime, end: str | datetime, symbol: str):
        self._start = start
        self._end = end
        self._symbol = symbol
        ticker = yf.Ticker(symbol)
        t = ticker.history(start=start, end=end, interval="1d")
        t.drop(columns=["Dividends", "Stock Splits"], inplace=True)
        t.rename(
            columns={"High": "high", "Low": "low", "Open": "open", "Close": "close", "Volume": "volume"}, inplace=True
        )
        t.index.name = "date"
        self._stock_data = t
        self.metrics = {}

    @contextmanager
    def with_end_date(self, end: str | datetime) -> Generator[DateLimitedAsset, Any, Any]:
        a = DateLimitedAsset(self._symbol, self._stock_data.loc[:end])
        yield a

    def get_metric[T: (int, float, bool, str)](self, key: str) -> Union[T, None]:
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


class DateLimitedAssets(Assets):
    def __init__(self, start: str | datetime, end: str | datetime, symbols: list[str]):
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
        return DataFrame()


class Assets(Assets):
    def __init__(self, start: str, end: str, symbols: list[str]):
        self._start = start
        self._end = end
        self._symbols = symbols
        self.metrics = {}

    @contextmanager
    def with_end_date(self, end: str | datetime) -> Generator[DateLimitedAssets, Any, Any]:
        a = DateLimitedAssets(self._start, end, self._symbols)
        yield a

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
        return DataFrame()
