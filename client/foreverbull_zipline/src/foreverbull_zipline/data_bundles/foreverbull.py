import os
import warnings

from datetime import date
from typing import Iterable
from typing import Tuple

import numpy as np
import pandas as pd

from pandas import read_sql_query
from zipline.utils.cli import maybe_show_progress

from foreverbull.models import get_engine
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.backtest.v1 import backtest_pb2


warnings.filterwarnings("ignore")


class DatabaseEngine:
    def __init__(self):
        database_uri = os.environ.get("DATABASE_URL")
        if database_uri is None:
            raise KeyError("DATABASE_URL environment variable not set")

        self._engine = get_engine(database_uri)

    def get_data(self, symbol, from_date, to_date):
        query = f"""SELECT open, high, low, close, volume, time FROM ohlc
        WHERE symbol='{symbol}' AND time BETWEEN '{from_date}' AND '{to_date}'
        ORDER BY time asc"""
        return read_sql_query(query, self._engine)


class SQLIngester:
    engine: DatabaseEngine
    symbols: list[str]
    from_date: date
    to_date: date

    def __init__(self, backtest: backtest_pb2.Backtest):
        self.backtest = backtest
        self.engine = DatabaseEngine()

    def create_metadata(self) -> pd.DataFrame:
        return pd.DataFrame(
            np.empty(
                0,
                dtype=[
                    ("start_date", "datetime64[ns]"),
                    ("end_date", "datetime64[ns]"),
                    ("auto_close_date", "datetime64[ns]"),
                    ("symbol", "object"),
                    ("exchange", "object"),
                ],
            )
        )

    def get_stock_data(self, symbol: str, from_date: date, to_date: date) -> pd.DataFrame:
        data = self.engine.get_data(symbol, from_date, to_date)
        data["time"] = pd.to_datetime(data["time"])
        data.rename(columns={"time": "Date"}, inplace=True, copy=False)
        data.set_index("Date", inplace=True)
        return data

    def writer(self, show_progress: bool) -> Iterable[Tuple[int, pd.DataFrame]]:
        def what_to_ingest():
            symbols = set(
                [asset.symbol for period in self.backtest.periods for asset in period.assets if asset.enabled]
            )
            symbols.add(self.backtest.benchmark)
            for symbol in symbols:
                yield (
                    pb_utils.from_proto_date_to_pydate(self.backtest.periods[0].start_date),
                    pb_utils.from_proto_date_to_pydate(self.backtest.periods[-1].end_date),
                    symbol,
                )

        with maybe_show_progress(self.backtest.periods, show_progress, label="Ingesting from SQL"):
            for index, to_ingest in enumerate(what_to_ingest()):  # type: ignore
                from_date, to_date, symbol = to_ingest
                data = self.get_stock_data(symbol, from_date, to_date)
                if len(data) == 0:
                    raise ValueError(f"No data found for {symbol} on {from_date} to {to_date}")
                data.dropna(
                    inplace=True
                )  # Yahoo can sometimes add duplicate rows on same date, one which is full or NaN
                start_date = data.index[0]
                end_date = data.index[-1]
                autoclose_date = end_date + pd.Timedelta(days=1)
                autoclose_date = None  # TODO: How to autoclose date?
                asset_row = pd.DataFrame(
                    {
                        "start_date": [start_date],
                        "end_date": [end_date],
                        "autoclose_date": [autoclose_date],
                        "symbol": [symbol],
                        "exchange": ["NASDAQ"],
                    }
                )
                self._df_metadata = pd.concat([self._df_metadata, asset_row], ignore_index=True)
                yield index, data

    def __call__(
        self,
        environ,
        asset_db_writer,
        minute_bar_writer,
        daily_bar_writer,
        adjustment_writer,
        calendar,
        start_session,
        end_session,
        cache,
        show_progress,
        output_dir,
    ):
        self._df_metadata = self.create_metadata()
        daily_bar_writer.write(self.writer(show_progress), show_progress=show_progress)
        asset_db_writer.write(equities=self._df_metadata)
        adjustment_writer.write()
