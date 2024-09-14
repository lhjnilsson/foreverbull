import os
import warnings
from datetime import datetime
from typing import Iterable, Tuple

import numpy as np
import pandas as pd
from foreverbull.models import get_engine
from pandas import read_sql_query
from zipline.data.bundles import register
from zipline.utils.cli import maybe_show_progress

warnings.filterwarnings("ignore")


class DatabaseEngine:
    def __init__(self):
        database_uri = os.environ.get("DATABASE_URL")
        if database_uri is None:
            raise KeyError("DATABASE_URI environment variable not set")

        self._engine = get_engine(database_uri)

    def get_data(self, symbol, from_date, to_date):
        query = f"""SELECT open, high, low, close, volume, time FROM ohlc
        WHERE symbol='{symbol}' AND time BETWEEN '{from_date}' AND '{to_date}'
        ORDER BY time asc"""
        return read_sql_query(query, self._engine)


class SQLIngester:
    engine: DatabaseEngine
    symbols: list[str]
    from_date: datetime
    to_date: datetime

    def __init__(self):
        pass

    def create_metadata(self) -> pd.DataFrame:
        return pd.DataFrame(
            np.empty(
                len(self.symbols),
                dtype=[
                    ("start_date", "datetime64[ns]"),
                    ("end_date", "datetime64[ns]"),
                    ("auto_close_date", "datetime64[ns]"),
                    ("symbol", "object"),
                    ("exchange", "object"),
                ],
            )
        )

    def get_stock_data(self, symbols: str) -> pd.DataFrame:
        data = self.engine.get_data(symbols, self.from_date, self.to_date)
        data["time"] = pd.to_datetime(data["time"])
        data.rename(columns={"time": "Date"}, inplace=True, copy=False)
        data.set_index("Date", inplace=True)
        return data

    def writer(self, show_progress: bool) -> Iterable[Tuple[int, pd.DataFrame]]:
        with maybe_show_progress(self.symbols, show_progress, label="Ingesting from SQL") as it:
            for index, symbol in enumerate(it):  # type: ignore
                data = self.get_stock_data(symbol)
                if len(data) == 0:
                    raise ValueError(f"No data found for {symbol} on {self.from_date} to {self.to_date}")
                data.dropna(
                    inplace=True
                )  # Yahoo can sometimes add duplicate rows on same date, one which is full or NaN
                start_date = data.index[0]
                end_date = data.index[-1]
                autoclose_date = end_date + pd.Timedelta(days=1)
                self._df_metadata.iloc[index] = start_date, end_date, autoclose_date, symbol, "NASDAQ"
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


register("foreverbull", SQLIngester())
