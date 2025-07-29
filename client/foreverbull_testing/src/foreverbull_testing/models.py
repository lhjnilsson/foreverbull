import tempfile

from dataclasses import dataclass
from datetime import UTC
from datetime import datetime
from typing import Any
from typing import Union

import pandas

from pandas import DataFrame
from pandas import read_sql_query
from sqlalchemy import Connection

from foreverbull import Asset as BaseAsset
from foreverbull import Assets as BaseAssets
from foreverbull import Portfolio
from foreverbull import models
from foreverbull.broker.storage import Storage
from foreverbull.foreverbull import BacktestSession
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb2


@dataclass
class Position:
    symbol: str
    amount: int


class Asset(BaseAsset):
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


class Assets(BaseAssets):
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
                marketdata_pb2.Position(
                    symbol=position.symbol,
                    amount=position.amount,
                )
            )

        if dt is not None:
            dt = dt.replace(tzinfo=UTC)  # Make sure its UTC, TODO: Replace datetime with date
        return Portfolio(
            marketdata_pb2.Portfolio(
                timestamp=pb_utils.to_proto_timestamp(dt) if dt else None,
                positions=proto_positions,
                portfolio_value=portfolio_value,
            ),
            self._db,
        )


class BacktestResult:
    def __init__(self, backtest: backtest_pb2.Backtest, execution_id: str):
        self._backtest = backtest
        self._execution_id = execution_id
        self.__df = None

    @property
    def dataframe(self) -> pandas.DataFrame:
        if self.__df is not None:
            return self.__df
        storage = Storage.from_environment()
        with tempfile.NamedTemporaryFile() as tmp:
            storage.download_object("results", self._execution_id, tmp.name)
            df = pandas.read_pickle(tmp.name)
            assert type(df) is pandas.DataFrame, "result is not a dataframe"
        self.__df = df
        return df


class Backtest:
    def __init__(self, backtest_session: BacktestSession):
        self._backtest_session = backtest_session

    def get_default(self) -> backtest_pb2.Backtest:
        return self._backtest_session.backtest

    def run_execution(self, algorithm: models.Algorithm, backtest: backtest_pb2.Backtest) -> BacktestResult:
        self._backtest_session._algorithm = algorithm
        periods = []
        exc_id: str | None = None
        for exc_id, period in self._backtest_session.run_execution(
            backtest.periods[0].start_date,
            backtest.periods[-1].end_date,
        ):
            periods.append(period)
        assert exc_id is not None, "no execution id"
        return BacktestResult(backtest, exc_id)
