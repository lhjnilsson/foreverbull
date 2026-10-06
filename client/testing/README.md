# foreverbull-testing

Pytest plugin used by Foreverbull algorithm projects. Cookiecutter templates under `client/templates/` are the supported way to start an algorithm; this package is what those projects use to unit-test strategy code before a full backtest.

Install it with the rest of the client workspace (`uv sync` from `client/`) or as a dependency of a generated project.

## Fixtures

The plugin registers itself as `foreverbull` (`pytest11`). None of the fixtures are autouse. Tests that do not execute SQL — CLI rendering, proto utils, and in-process namespace socket fakes — must not request `fb_database`, `asset_manager`, `portfolio_manager`, or `session`, and will not start Docker.

| Fixture | Scope | Docker | Purpose |
| --- | --- | --- | --- |
| `fb_env` | function | no | Process env defaults (`setup_env`) |
| `namespace_server` | function | no | In-process pynng namespace fake on an ephemeral port |
| `fb_database` | session | yes | Postgres Testcontainer plus `verify_or_populate` for OHLC data |
| `asset_manager` | function | yes | Load `Asset` / `Assets` windows from that database |
| `portfolio_manager` | function | yes | Build a `Portfolio` with optional positions |
| `session` | function | yes | Insert synthetic OHLC bars (`constant`, `trend`, `cross_up`), call production `Algorithm.process`, and `replay` that order trace |
| `backtest` | function | no | Optional broker run via `create_session` when `--backtest <name>` is passed; otherwise skipped |

`fb_database` starts (or reuses) a Docker Postgres named `fbull_client_testing_pg` and can download prices through yfinance when a window is missing. `session` inserts `ohlc` rows through that engine and does not call yfinance. `Session.constant`, `Session.trend`, and `Session.cross_up` build weekday bars (open, high, and low equal close, volume 1) and replace that symbol's rows on the test connection. `expected_shares(portfolio_value, percent, close)` is `int(portfolio_value * percent / close)`. `Session.replay(algorithm, as_of_dates, portfolio_value)` walks those dates. For each day it calls production `Algorithm.process` with that day's as_of and the carried positions. Pending `order.amount` is a delta, not a target: add it to the position and fill at that day's close. There is no slippage and no commission. Cash starts equal to `portfolio_value` when there are no positions. The next portfolio value is cash plus the sum of shares times that day's close. That result is an order trace for tests, not the broker or Zipline result. `database.populate` remains the downloader for `verify_or_populate`. Do not switch SQL tests to SQLite. Do not convert these tests to Kind.

## Example

```python
from datetime import datetime

from foreverbull_testing.models import AssetManager
from foreverbull_testing.models import PortfolioManager
from foreverbull_testing.models import Position


def test_should_buy(asset_manager: AssetManager, portfolio_manager: PortfolioManager):
    asset = asset_manager.get_asset(datetime(2021, 1, 1), datetime(2021, 3, 31), "AAPL")
    portfolio = portfolio_manager.get_portfolio(
        datetime(2021, 3, 31),
        positions=[Position(symbol="AAPL", amount=10)],
    )
    assert asset.symbol == "AAPL"
    assert portfolio.positions["AAPL"] is not None
```

Namespace-only tests request `namespace_server` (and optionally `fb_env`) instead of `fb_database`.

## Live backtests

`--backtest` is optional. Without `--backtest <name>`, the `backtest` fixture skips with `pass --backtest <name> to run live backtest tests against a broker`. Package tests do not pass the flag, so they stay local and do not need a broker or Kind.

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
