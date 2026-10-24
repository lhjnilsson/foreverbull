# Schema baseline

One Postgres schema, still applied by the existing `CREATE TABLE` statements and idempotent `ALTER`s. Goose is not part of this work. `pkg/backtest/universe.md` still owns identity: `ohlc` stays on `symbol`. `asset_symbol` is the dated ticker alias.

This file is the contract. Do not drop bars, do not move bars onto ISIN, and do not store trading secrets.

## One definition

`pkg/strategy/internal/repository/storage.go` creates parent tables so strategy tests can attach foreign keys. Those copies must match the tables created while `AUTH_ENABLED` is false:

- `backtest` includes `provider text NOT NULL DEFAULT 'yahoo'` and `profile text NOT NULL DEFAULT ''`.
- `backtest_execution` includes `function_arguments jsonb` and `provider text NOT NULL DEFAULT 'yahoo'`.
- `instrument` and `asset_symbol` are the same SQL in `pkg/finance/internal/repository/instrument.go` and `pkg/backtest/internal/repository/instrument.go`.

The enable boot alters the live `service`, `backtest`, and `session` tables and does not change `parentTables`. See [Ownership](#ownership).

`CREATE TABLE IF NOT EXISTS` does not change an existing table. Each new constraint also has an idempotent migration function, called from the same startup path as `MigrateOHLC`. Running it twice is safe. If existing rows would violate the new constraint, return an error that names the table. Do not delete those rows.

Create `btree_gist` next to `uuid-ossp` (`CREATE EXTENSION IF NOT EXISTS btree_gist`).

## Bars

`ohlc.open`, `high`, `low`, `close`, and `volume` are `NOT NULL`.

```sql
CHECK (high >= low AND volume >= 0)
CHECK (provider IN ('yahoo', 'stooq', 'alpaca'))
```

`backtest.provider` gets the same provider check. Leave `ohlc.time` as `timestamp`. Leave the unique key on `(provider, symbol, time)`.

## Identity intervals

`asset_symbol.isin` references `instrument(isin)`. `universe_member.isin` references `instrument(isin)`.

Intervals are half-open `[from_date, to_date)`. A null `to_date` is unbounded. Overlaps for one ISIN are rejected:

```sql
EXCLUDE USING gist (
  isin WITH =,
  daterange(from_date, to_date, '[)') WITH &&
)
```

`universe_member` adds `backtest_name WITH =` to that exclusion, so two backtests may hold the same ISIN on the same dates. That column is `backtest_name` while `AUTH_ENABLED` is false. The enable boot rebuilds `universe_member_no_overlap` on `backtest_id` instead. See [Ownership](#ownership).

Index `asset_symbol (symbol, from_date)`.

## Runs

While `AUTH_ENABLED` is false, `session.backtest` is `NOT NULL` and references `backtest(name)`.

`backtest_execution.provider` is `NOT NULL`. `Create` copies it from the backtest. While `session.backtest` exists, a migration backfills existing rows with:

```sql
UPDATE backtest_execution e
SET provider = b.provider
FROM session s
JOIN backtest b ON b.name = s.backtest
WHERE e.session = s.id AND e.provider IS NULL
```

Then set the default `yahoo` for any row the join cannot fill, then set `NOT NULL` and the provider check. If `session.backtest` is gone, skip that update. If `provider` is already `NOT NULL` and `backtest_execution_provider_check` exists, return. `backtest_execution` does not gain a new column. After the enable boot, `Create` joins `backtest b ON b.id = s.backtest_id`.

`backtest_execution_period.backtest_execution` references `backtest_execution(id)` on delete cascade. The unique key on `(backtest_execution, date)` stays.

While `AUTH_ENABLED` is false, `strategy_experiment.backtest_name` references `backtest(name)`. The enable boot replaces that foreign key with `backtest_id` referencing `backtest(id)` and drops `backtest_name`. `MigrateExperimentBacktest` does not re-add the name foreign key once `backtest_name` is gone, or once `backtest_id` already references `backtest(id)`.

## Ownership

📝 These keys are applied only on the boot where `AUTH_ENABLED=true`, in that process, before it accepts requests. The migration does not run while the flag is false. There is no Helm hook. `CREATE TABLE IF NOT EXISTS` on the false path keeps today's primary keys. `ohlc` stays per `(provider, symbol, time)`.

`service.id` becomes the primary key. `service.owner_id` is `NOT NULL` and references `account(id)` on delete restrict. Uniqueness is `(owner_id, image)`. `service_status.service_id` and `service_instance.service_id` reference `service(id)`. Image is no longer the key.

`backtest.id` becomes the primary key. `backtest.owner_id` is `NOT NULL` and references `account(id)` on delete restrict. Uniqueness is `(owner_id, name)`. The non-empty name check and the provider check stay. `name` is not the primary key. Ingestion objects are copied from the name key to the id key before that primary key is dropped. A failed copy leaves the name key and the name primary key.

Child foreign keys move onto `backtest.id`:

- `session.backtest_id` replaces `session.backtest`.
- `universe_member.backtest_id`. Primary key `(backtest_id, isin, from_date)`. `universe_member_no_overlap` keeps its name and uses `backtest_id`.
- `strategy_experiment.backtest_id` replaces `backtest_name`.
- `backtest_period` primary key `(backtest_id, start_date, end_date)`.
- `backtest_period_asset` primary key `(backtest_id, start_date, end_date, symbol)`, referencing that period key.

`strategy.id` stays the primary key. `strategy.owner_id` is `NOT NULL`. Uniqueness is `(owner_id, name)`, replacing the global name unique. `strategy_version.service_id` references `service(id)`. `service_image` stays text and is not a foreign key.

`owner_id` is the owner of that row. It is not a grant level. Delete of the service, backtest, or strategy is that `owner_id` when the global role is `editor`, or `admin` on any row.

After `owner_id` is `NOT NULL` on `service`, `backtest`, or `strategy`, a start with `AUTH_ENABLED` false exits. That is not a rollback.

## Status history

`service_status` and `service_instance_status` gain `id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY`. Trigger inserts omit `id`. Existing rows receive ids. Do not add a second status row from the migration itself.

## Tests

Repository tests must show:

- An `ohlc` row with a null close, `high < low`, a negative volume, or provider `nasdaq` is rejected.
- Two `asset_symbol` rows for one ISIN with overlapping dates are rejected. A later interval that starts on the previous `to_date` is accepted.
- The same rule for `universe_member` inside one backtest.
- `backtest_execution.provider` equals the backtest provider after create.
- `service_status` has a primary key, and inserting a status still notifies.

## Out of scope

- Goose or any other migration runner.
- Rewriting `ohlc` onto ISIN, or changing `ohlc.time` to `date`.
- Trading profile tables, secrets, or a second Postgres schema.
- Replacing `uuid_generate_v4()`.

## Definition of done

- The strategy parent-table copy matches `backtest` and `backtest_execution`.
- Both `asset_symbol` definitions match, including the foreign key, the exclusion constraint, and the symbol index.
- The checks, the session column, the execution provider, the period foreign key, the experiment foreign key, and both status primary keys exist as idempotent SQL.
- A second run of each migration function succeeds.
- The ownership migration, including `owner_id` and the uuid primary keys, runs only on the `AUTH_ENABLED=true` boot. A second run succeeds. The false path does not apply it.
- `pkg/backtest/universe.md` still says `ohlc` stays on `symbol`.
