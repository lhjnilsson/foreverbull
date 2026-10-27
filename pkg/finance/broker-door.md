# 📝 Broker as the only door — OHLC / positions / results

Client code talks to the broker. It does not receive a database URL or open
Postgres for normal backtest and paper flows. This file is the short API
surface that replaces those direct reads. Implementation that contradicts it
is wrong.

Parent epic: broker as the only door. Workers read `WorkerRequest.bars`.
The broker leaves `ExecutionConfiguration.databaseURL` empty. This supersedes
planning leaf #33 (broker-mediated access for backtest and paper). Multi-tenant
auth stays a separate change.

## Gap map

| Client need today | Direct DB today | Broker path |
| --- | --- | --- |
| `Asset.stock_data` / `Assets.stock_data` (last 252 bars, `time <= as_of`, provider) | `SELECT` on `ohlc` | `WorkerRequest.bars` (alias-resolved). `Marketdata.GetOHLC` stays symbol-exact for CLI and gRPC clients |
| `Portfolio._close_on_bar` (close on bar day) | `SELECT close FROM ohlc` | The matching bar in `WorkerRequest.bars` |
| Connectivity check at configure | `SELECT 1 FROM asset` | Dropped. Configure does not open a database |
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

The pynng worker request carries the lookback bars for that step. The worker
builds `Asset` / `Assets` / close lookups from those bars and does not open
a database. An empty list means no stored bars for the step.

The broker fills `bars` with alias-resolved rows (FB then META is one ISIN,
labeled with the symbol the algorithm asked for). The window matches the
client filter: `since < time <= as_of`, at most 252 bars per symbol.
Backtests use the backtest provider. Paper runs use `yahoo`. `GetOHLC`
stays symbol-exact for CLI, tests, and any caller that is already a
broker gRPC client.

## ExecutionConfiguration.databaseURL

Deprecated. Field number 3 stays for wire compatibility. The broker leaves
it empty. A client ignores a URL an older broker still sets and does not
open Postgres.

## Out of scope here

- Changing Grafana; it already uses `GetExecution`
- Auth / multi-tenant row filters (separate security work)
- Reproducible backtest pins (separate issue)
- Dropping the unused `foreverbull_worker` role and `WORKER_DATABASE_URL`
  on the broker pod

## Checks

- `GetOHLC` with one symbol and `limit = 1` returns the latest bar at or
  before `as_of` for that provider, or an empty list
- `GetOHLC` with several symbols returns at most `limit` bars each
- Empty `symbols` or missing `as_of` is `InvalidArgument`
- `limit > 2520` or more than 500 symbols is `InvalidArgument`
- Response and logs never contain a Postgres URL or password
