# 📝 Reproducible backtest runs

Same strategy + data + config should produce comparable results. This file is
the re-run contract for strategists and bots. Implementation that contradicts
it is wrong.

Parent: strategy-ready foundation. Sibling work owns universe/OHLC trust,
metrics presentation, and the write→run→read walkthrough.

## Re-run story

To compare two runs (or re-run one for a check):

1. **Keep the same backtest.** Use the existing backtest name. Do not recreate
   from an ETF or YAML unless you intend a new universe. Membership, benchmark,
   and the ingested symbol set live on that backtest.
2. **Keep the same date range.** Pass the same `--start` / `--end`, or omit both
   to use the full backtest range. Dates are stored on each execution.
3. **Keep the same price provider.** `yahoo`, `stooq`, or `alpaca` is copied
   onto the execution at create time. Read it back with
   `GetExecution` / `fbull backtest execution <id>` — do not guess from a
   recreated backtest.
4. **Keep the same algorithm inputs.** Worker function parameters used for the
   run are stored as `functions` on `GetExecution`. A set value wins, including
   an explicit empty string; otherwise the default is what was stored. Re-run
   with those same values.
5. **Keep the same strategy code.** For `fbull backtest run <backtest> <file>`,
   use the same file contents. For a registered strategy, use the same
   `StrategyVersion` (same `service_image` and version parameters). The local
   file path is not stored on the execution.
6. **Reuse stored bars.** Comparable results assume the OHLC rows already in
   the broker for that provider. Re-downloading from a live feed may change
   bars. See the gap map.

Inspect a finished run:

```bash
fbull backtest execution <execution-id>
```

That prints the pinned inputs (backtest, dates, provider, function parameters)
and the period metrics. Then re-run:

```bash
# Local algorithm file — same backtest, same dates, same file contents
fbull backtest run <backtest-name> path/to/strategy.py --start YYYY-MM-DD --end YYYY-MM-DD

# Or omit --start/--end to use the whole backtest range again
fbull backtest run <backtest-name> path/to/strategy.py
```

For a registered strategy experiment, create another experiment on the **same**
version and **same** backtest name; do not bump the version unless the code
change is intentional.

## What must be fixed vs what may drift

| Input | Must match for a fair compare | Where it is pinned today |
| --- | --- | --- |
| Backtest universe (periods, members, benchmark) | Yes | `backtest` + period/asset tables; identify by backtest name |
| Run start / end dates | Yes | `backtest_execution.start_date` / `end_date` |
| Price provider | Yes | `backtest_execution.provider` (also on `GetExecution.provider`) |
| Algorithm function parameters | Yes | `backtest_execution.function_arguments` → `GetExecution.functions` |
| Strategy source / image | Yes | Local file contents (not hashed on the execution); or `StrategyVersion.service_image` + parameters |
| Stored OHLC for that provider | Yes, for bit-stable numbers | Broker `ohlc` rows; not versioned per execution |
| Zipline / broker image digest | Ideal, not required for “comparable” | Not stored on the execution (may drift across deploys) |
| Floating image tags (`:latest`, mutable tags) | Avoid | Strategy versions should prefer immutable tags/digests |
| Live Yahoo / Stooq / Alpaca if re-ingested | May drift | External feeds change; do not re-download when comparing |
| Metric list / Grafana layout | Presentation only | Sibling metrics work; numbers on periods stay |

“Comparable” means the same decisions and portfolio path under the same bars
and knobs — not a cryptographic bit-identical binary across every deploy.

## What CreateExecution already pins

On create, the broker stores:

- session → backtest identity
- start_date, end_date (request, or full backtest range when omitted)
- provider (copied from the backtest; column has no default)
- function_arguments (values actually used; see `executionFunctionsFromAlgorithm`)

`GetExecution` returns those pins as `execution`, `functions`, and `provider`.
Broker port, namespace port, and database URL are never stored on the execution.

## Gap map

| Gap | Effect on re-runs | Owner / sibling |
| --- | --- | --- |
| OHLC / universe correctness (empty days, renames, feed quirks) | Wrong members or bars even when config matches | #66 (Universe & OHLC checklist) |
| First-class compare metrics after a run | Hard to judge “same” without tribal Grafana knowledge | #67 (Strategy metrics) |
| Linear write → run → read doc | Bots miss install or CLI steps | #68 (bot-friendly path) |
| No content hash of the local strategy file on the execution | File path alone cannot prove code match | Software Engineer (follow-up; strategy versions already pin `service_image`) |
| Zipline / broker image digest not on the execution | Engine upgrades can move fills or calendar edge cases | Software Engineer (follow-up; prefer immutable image refs in deploy) |
| Re-create from ETF pulls fresh iShares snapshots | New backtest ≠ old membership | Avoid recreate; keep the existing backtest (universe contract: `universe.md`) |
| Client still able to read OHLC via DB URL | Side channel, not a pin | #64 / broker-door (public PR); not this contract |

## Out of scope here

- Changing Zipline fill logic or adding a RNG seed API
- Snapshotting full OHLC blobs onto each execution
- The product install / nightly loop
- Removing the client DB URL

## Checks

- `CreateExecution` persists provider from the backtest and the used function arguments
- `GetExecution` returns `functions` and `provider` for that execution id
- `fbull backtest execution <id>` prints provider and function parameters when present
- Re-running the same backtest name + dates + algorithm inputs against stored bars is the documented compare path
