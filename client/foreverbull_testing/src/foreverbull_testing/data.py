from dataclasses import dataclass
from datetime import datetime
from typing import Any
from typing import Union

import pandas

from pandas import DataFrame
from pandas import read_sql_query
from sqlalchemy import Connection

from foreverbull import Asset  # type: ignore
from foreverbull import Assets  # type: ignore
from foreverbull import Portfolio
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance import finance_pb2


@dataclass
class Position:
    symbol: str
    amount: int


class Asset(Asset):
    def __init__(
        self, db: Connection, start: str | datetime, end: str | datetime, symbol: str, metrics: dict[str, Any] = {}
    ):
        self._start = start
        self._end = end
        self._symbol = symbol
        self._stock_data = read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time BETWEEN '{start}' AND '{end}'
            AND symbol='{symbol}'""",
            db,
        )
        self.metrics = metrics

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
    def __init__(
        self,
        db: Connection,
        start: str | datetime,
        end: str | datetime,
        symbols: list[str],
        metrics: dict[str, Any] = {},
    ):
        self._db = db
        self._start = start
        self._end = end
        self._symbols = symbols
        self._stock_data = read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time BETWEEN '{start}' AND '{end}' AND symbol IN {tuple(symbols) if len(symbols) > 1 else f"('{symbols[0]}')" }""",
            db,
        )
        self._stock_data.set_index(["symbol", "time"], inplace=True)
        self._stock_data.sort_index(inplace=True)
        self.metrics = metrics

    def get_metrics[T: (int, float, bool, str)](self, key: str) -> pandas.Series:
        try:
            return pandas.Series(self.metrics[key])
        except KeyError:
            return pandas.Series()

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


class AssetManager:
    def __init__(self, db: Connection, vop):
        self._db = db
        self._vop = vop

    def get_asset(self, start: str | datetime, end: str | datetime, symbol: str) -> Asset:
        self._vop(None, start, end, [symbol])
        return Asset(self._db, start, end, symbol)

    def get_assets(
        self, start: str | datetime, end: str | datetime, symbols: list[str], metrics: dict[str, Any] = {}
    ) -> Assets:
        self._vop(None, start, end, symbols)
        return Assets(self._db, start, end, symbols, metrics)


class PortfolioManager:
    def __init__(self, db: Connection):
        self._db = db

    def get_portfolio(
        self, dt: datetime | None = None, portfolio_value: float = 100_000, positions: list[Position] = []
    ) -> Portfolio:
        proto_positions = []
        for position in positions:
            proto_positions.append(
                finance_pb2.Position(
                    symbol=position.symbol,
                    amount=position.amount,
                )
            )
        return Portfolio(
            finance_pb2.Portfolio(
                timestamp=pb_utils.to_proto_timestamp(dt) if dt else None,
                positions=proto_positions,
                portfolio_value=portfolio_value,
            ),
            self._db,
        )
