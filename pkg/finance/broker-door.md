# 📝 Broker as the only door — OHLC / positions / results

Client code talks to the broker. It does not receive a database URL or open
Postgres for normal backtest and paper flows. This file is the short API
surface that replaces those direct reads. Implementation that contradicts it
is wrong.

Parent epic: broker as the only door. This contract unblocks removing the
client DB URL and dropping client DB drivers.

## Gap map

| Client need today | Direct DB today | Broker path |
| --- | --- | --- |
| `Asset.stock_data` / `Assets.stock_data` (last 252 bars, `time <= as_of`, provider) | `SELECT` on `ohlc` | `Marketdata.GetOHLC` |
| `Portfolio._close_on_bar` (close on bar day) | `SELECT close FROM ohlc` | Same `GetOHLC` with `limit = 1`, or the matching bar in `WorkerRequest.bars` |
| Connectivity check at configure | `SELECT 1 FROM asset` | Dropped once the worker no longer opens a DB |
| Positions during a run | None (already on the wire) | `WorkerRequest.portfolio.positions` |
| Positions / portfolio outside a worker | None for paper session | `TradingServicer.GetPortfolio` |
| Positions on a finished period | None | `Period.positions` on `BacktestServicer.GetExecution` |
| Execution results (PNL, returns, equity curve, …) | None for Grafana/CLI | `BacktestServicer.GetExecution` (execution + periods) |

Positions and results already go through the broker. The only new RPC is
stored-bar OHLC read. Download from Yahoo/Stooq/Alpaca stays
`DownloadHistoricalData`; that path writes bars, it does not serve them to
algorithms.

## Marketdata.GetOHLC

Reads bars already stored in the broker database. Does not call an external
price feed.

| Field | Rule |
| --- | --- |
| `symbols` | Required, non-empty, at most 500 |
| `as_of` | Required. Inclusive upper bound on bar time |
| `provider` | `yahoo`, `stooq`, or `alpaca`. Empty means `yahoo` |
| `limit` | Per-symbol cap. `0` means 252. Above 2520 is `InvalidArgument` |

Response bars are ordered by symbol ascending, then time ascending. A symbol
with no stored bars in range is omitted (no error). Errors do not include a
database URL or password.

Go entry: `pkg/finance/internal/api.MarketdataServer.GetOHLC`. Storage:
`repository.OHLC.List`.

## WorkerRequest.bars

Optional field on the pynng worker request. When non-empty, the worker must
not open a database for OHLC; it builds `Asset` / `Assets` / close lookups
from those bars. When empty, today's `databaseURL` path still works until
that path is removed.

The broker fills `bars` from the same stored rows `GetOHLC` reads (same
lookback and provider as the backtest). Prefer this push for the worker so
strategy processes never dial Postgres and do not need a Marketdata gRPC
client. `GetOHLC` remains for CLI, tests, and any caller that is already a
broker gRPC client.

## ExecutionConfiguration.databaseURL

Deprecated for client use. Field number 3 stays for wire compatibility until
workers stop reading it. Brokers may still set it during the transition;
clients must not require it once `WorkerRequest.bars` is always filled.

## Out of scope here

- Removing the client DB URL and SQLAlchemy/psycopg deps (follow-up
  implementation)
- Changing Grafana; it already uses `GetExecution`
- Auth / multi-tenant row filters (separate security work)
- Reproducible backtest pins (separate issue)

## Checks

- `GetOHLC` with one symbol and `limit = 1` returns the latest bar at or
  before `as_of` for that provider, or an empty list
- `GetOHLC` with several symbols returns at most `limit` bars each
- Empty `symbols` or missing `as_of` is `InvalidArgument`
- `limit > 2520` or more than 500 symbols is `InvalidArgument`
- Response and logs never contain a Postgres URL or password
