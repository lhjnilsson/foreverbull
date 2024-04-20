import logging
import re
from datetime import datetime

from pandas import DataFrame, read_sql_query
from sqlalchemy import create_engine, engine


# Hacky way to get the database URL, TODO: find a better way
def get_engine(
    url: str,
):
    log = logging.getLogger(__name__)

    if url.startswith("postgres://"):
        url = url.replace(
            "postgres://",
            "postgresql://",
            1,
        )

    try:
        engine = create_engine(url)
        engine.connect()
        return engine
    except Exception as e:
        log.warning(f"Could not connect to {url}: {e}")

    for hostname in [
        "localhost",
        "postgres",
        "127.0.0.1",
    ]:
        try:
            database_port = re.search(
                r":(\d+)/",
                url,
            ).group(1)
            url = url.replace(
                f":{database_port}",
                ":5432",
                1,
            )
            database_host = re.search(
                r"@([^/]+):",
                url,
            ).group(1)
            url = url.replace(
                f"@{database_host}:",
                f"@{hostname}:",
                1,
            )
            engine = create_engine(url)
            engine.connect()
            return engine
        except Exception as e:
            log.warning(f"Could not connect to {url}: {e}")
    raise Exception("Could not connect to database")


class Asset:
    def __init__(
        self,
        symbol: str,
        as_of: datetime,
        db: engine.Connection,
    ):
        self.symbol = symbol
        self._as_of = as_of
        self._db = db

    @property
    def stock_data(
        self,
    ) -> DataFrame:
        return read_sql_query(
            f"""Select symbol, time, high, low, open, close, volume
            FROM ohlc WHERE time <= '{self._as_of}' AND symbol='{self.symbol}'""",
            self._db,
        )
