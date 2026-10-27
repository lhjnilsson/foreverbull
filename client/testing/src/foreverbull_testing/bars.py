"""Stored bars as OHLC messages for algorithm tests.

SQL stays in this package. The client builds frames from the messages.
"""

from collections.abc import Sequence
from datetime import UTC
from datetime import datetime

from sqlalchemy import Connection
from sqlalchemy import bindparam
from sqlalchemy import text

from foreverbull.models.finance import ohlc_provider
from foreverbull.pb import pb_utils
from foreverbull.pb.foreverbull.finance.v1 import marketdata_pb


_SELECT_SYMBOLS = text(
    """
    SELECT symbol, time, open, high, low, close, volume
    FROM ohlc
    WHERE provider = :provider AND symbol IN :symbols AND time <= :as_of
    ORDER BY symbol ASC, time ASC
    """
).bindparams(bindparam("symbols", expanding=True))

_SELECT_ALL = text(
    """
    SELECT symbol, time, open, high, low, close, volume
    FROM ohlc
    WHERE provider = :provider AND time <= :as_of
    ORDER BY symbol ASC, time ASC
    """
)


def messages(
    connection: Connection,
    as_of: datetime | None,
    symbols: Sequence[str] | None = None,
) -> list[marketdata_pb.OHLC]:
    """Bars at or before as_of. symbols None reads every symbol for the provider."""
    if as_of is None:
        return []
    when = as_of if as_of.tzinfo is None else as_of.astimezone(UTC)
    stored = when.replace(tzinfo=None)
    provider = ohlc_provider()
    if symbols is not None:
        if not symbols:
            return []
        rows = connection.execute(
            _SELECT_SYMBOLS,
            {"provider": provider, "symbols": list(symbols), "as_of": stored},
        )
    else:
        rows = connection.execute(_SELECT_ALL, {"provider": provider, "as_of": stored})
    return [_message(row) for row in rows]


def _message(row) -> marketdata_pb.OHLC:
    when = row.time
    if when.tzinfo is None:
        when = when.replace(tzinfo=UTC)
    else:
        when = when.astimezone(UTC)
    return marketdata_pb.OHLC(
        symbol=str(row.symbol),
        timestamp=pb_utils.to_proto_timestamp(when),
        open=float(row.open),
        high=float(row.high),
        low=float(row.low),
        close=float(row.close),
        volume=int(row.volume),
    )
