# foreverbull-testing

Pytest plugin used by Foreverbull algorithm projects. Cookiecutter templates under
`client/templates/` are the supported way to start an algorithm; this package is
what those projects use to unit-test strategy code before a full backtest.

Install it with the rest of the client workspace (`uv sync` from `client/`) or as
a dependency of a generated project.

## Fixtures

The plugin registers itself as `foreverbull` (`pytest11`). Tests can request:

| Fixture | Scope | Purpose |
| --- | --- | --- |
| `fb_database` | session | Postgres Testcontainer plus `verify_or_populate` for OHLC data |
| `asset_manager` | function | Load `Asset` / `Assets` windows from that database |
| `portfolio_manager` | function | Build a `Portfolio` with optional positions |
| `backtest` | function | Live broker run; skipped unless `--backtest <name>` is passed |

`fb_database` starts (or reuses) a Docker Postgres named `fbull_client_testing_pg`
and can download prices through yfinance when a window is missing.

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

## Live backtests

Pass `--backtest <name>` to opt into tests that talk to a running Foreverbull
broker. Without that flag, those tests skip so unit tests stay local.

```bash
uv run pytest --backtest demo
```

Do not enable live trading from this plugin.

## Related

- Simple template (no TA-Lib): `client/templates/simple`
- Advanced template (TA-Lib): `client/templates/advanced`
- Generate and test templates from `client/`: `task template:test`
