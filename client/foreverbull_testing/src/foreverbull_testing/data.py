from datetime import datetime
from typing import Union

from pandas import DataFrame
from pandas import read_sql_query
from sqlalchemy import Connection

from foreverbull import Asset  # type: ignore
from foreverbull import Assets  # type: ignore


class Asset(Asset):
    def __init__(self, db: Connection, start: str | datetime, end: str | datetime, symbol: str):
        self._start = start
        self._end = end
        self._symbol = symbol
        self._stock_data = read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time BETWEEN '{start}' AND '{end}'
            AND symbol='{symbol}'""",
            db,
        )
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


class Assets(Assets):
    def __init__(self, db: Connection, start: str | datetime, end: str | datetime, symbols: list[str]):
        self._db = db
        self._start = start
        self._end = end
        self._symbols = symbols
        self._stock_data = read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time BETWEEN '{start}' AND '{end}' AND symbol IN {tuple(symbols)}""",
            db,
        )
        self._stock_data.set_index(["symbol", "time"], inplace=True)
        self._stock_data.sort_index(inplace=True)
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
            yield Asset(self._db, self._start, self._end, symbol)

    @property
    def stock_data(self) -> DataFrame:
        return self._stock_data
