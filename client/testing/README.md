# foreverbull-testing

Pytest plugin used by Foreverbull algorithm projects. Cookiecutter templates under `client/templates/` are the supported way to start an algorithm; this package is what those projects use to unit-test strategy code before a full backtest.

Install it with the rest of the client workspace (`uv sync` from `client/`) or as a dependency of a generated project.

## Fixtures

The plugin registers itself as `foreverbull` (`pytest11`). None of the fixtures are autouse. `fb_database` is not autouse. Tests that do not execute SQL — CLI rendering, proto utils, and in-process namespace socket fakes — must not request `fb_database`, `asset_manager`, `portfolio_manager`, or `session`, and will not start Docker.

| Fixture | Scope | Docker | Purpose |
| --- | --- | --- | --- |
| `fb_env` | function | no | Process env defaults (`setup_env`) |
| `namespace_server` | function | no | In-process pynng namespace fake on an ephemeral port |
| `fb_database` | session | yes | Postgres Testcontainer plus `verify_or_populate` for OHLC data |
| `asset_manager` | function | yes | Load the start/end testing `Asset` / `Assets` subclasses from that database |
| `portfolio_manager` | function | yes | Build a `Portfolio` with optional positions |
| `session` | function | yes | Insert synthetic OHLC bars (`constant`, `trend`, `cross_up`), call production `Algorithm.process`, and `replay` that order trace |
| `backtest` | function | no | Optional broker run via `create_session` when `--backtest <name>` is passed; otherwise skipped |

`fb_database` starts (or reuses) a Docker Postgres named `fbull_client_testing_pg`. SQL stays on Postgres. Do not switch SQL tests to SQLite. Do not convert these tests to Kind.

Yahoo populate is an explicit call. `database.populate` downloads bars with yfinance, and `verify_or_populate` calls it only when a requested window is missing. `session` does not call it. `Session.constant`, `Session.trend`, and `Session.cross_up` build weekday bars (open, high, and low equal close, volume 1) and replace that symbol's rows on the test connection. `session.assets` and `session.portfolio` return the worker types `Algorithm.process` builds. `asset_manager` still returns the start/end testing subclasses for callers that request it. Prefer `session` for new tests. `expected_shares(portfolio_value, percent, close)` is `int(portfolio_value * percent / close)`. `Session.run` calls production `Algorithm.process` for one as_of. `Session.replay(algorithm, as_of_dates, portfolio_value)` walks those dates. For each day it calls production `Algorithm.process` with that day's as_of and the carried positions. Pending `order.amount` is a delta, not a target: add it to the position and fill at that day's close. Replay fills at the close and is not the platform backtest. There is no slippage and no commission. Cash starts equal to `portfolio_value` when there are no positions. The next portfolio value is cash plus the sum of shares times that day's close. That result is an order trace for tests, not the broker or Zipline result.

## Example

```python
from foreverbull_testing.session import Session

from foreverbull import Algorithm
from foreverbull import Asset
from foreverbull import Function
from foreverbull import Portfolio


def handle_data(asset: Asset, portfolio: Portfolio) -> None:
    portfolio.order_target(asset.symbol, 1)


def test_run_orders_the_synthetic_bar(session: Session):
    algorithm = Algorithm(functions=[Function(callable=handle_data)])
    times = session.constant("SYN", 1, 25.0)
    orders = session.run(algorithm, times[-1], 100_000)
    assert len(orders) == 1
    assert orders[0].symbol == "SYN"
    assert orders[0].amount == 1
```

Namespace-only tests request `namespace_server` (and optionally `fb_env`) instead of `fb_database`.

## Live backtests

`--backtest` skips unless the flag is passed, and it does not place live orders. Without `--backtest <name>`, the `backtest` fixture skips with `pass --backtest <name> to run live backtest tests against a broker`. Package tests do not pass the flag, so they stay local and do not need a broker or Kind.

```bash
uv run pytest --backtest demo
```

With a name, the fixture looks the backtest up with `foreverbull.broker.backtest.get`. If the broker is unreachable or the name is missing, the test skips with that reason and does not error. If the broker returns the backtest, the fixture opens `broker.backtest.create_session`, connects with `Foreverbull.backtest_session`, and yields the existing `foreverbull_testing.models.Backtest`. Runs go through `Backtest.run_execution`. The fixture does not call `trading_session` or `PlaceOrder`, and it does not set `TRADING_ENABLED`. The broker is the Kind/Helm stack (`task build`, then `fbenv create` and `fbenv start`), not this plugin.

## Tests for this package

From `client/`:

```bash
uv run pytest testing/
```

## Related

- Simple template (no TA-Lib): `client/templates/simple`
- Advanced template (TA-Lib): `client/templates/advanced`
- Generate and test templates from `client/`: `task template:test`
