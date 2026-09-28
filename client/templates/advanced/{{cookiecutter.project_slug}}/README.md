# {{ cookiecutter.project_slug }}

Generated from the Foreverbull **advanced** cookiecutter template.
The strategies use [TA-Lib](https://ta-lib.org/) (`macd.py` and `rsi.py`).

Both size from `portfolio_value`: they leave a `cash_reserve` (default `0.05`),
cap the book at `max_positions` (default `10`), equal-weight deployable capital
across the names that qualify this bar, rebalance those targets every bar, and
flatten names that leave the set. Do not buy a fixed share lot. RSI ranks
oversold names and concentrates remaining deployable if fewer than
`max_positions` qualify. MACD ranks positive MACD the same way. Lookbacks and
risk knobs are `Function` parameters (`timeperiod` / `oversold`, MACD periods,
`max_positions`, `cash_reserve`).

Cookiecutter plus `foreverbull-testing` is the supported way to write algorithms.

Install TA-Lib (the C library and the `TA-Lib` Python package) before running
the strategies. If you do not have TA-Lib, use the simple template instead.

## Requirements

- Python 3.12+ and [uv](https://docs.astral.sh/uv/) (preferred; pip works)
- TA-Lib
- A running Foreverbull broker (Kind locally via `fbenv`, or Helm remote)

## Install

Skip this section if you already installed dependencies during generation.

```bash
uv sync
```

The hook also writes `requirements.txt`. Without uv:

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

If this repo is checked out locally, `uv sync` in `client/` puts `fbull` and `fbenv` on the workspace PATH (`uv run fbull`, `uv run fbenv`).

## Run against Kind / fbenv

The platform is Kubernetes, not Docker Compose. From a Foreverbull clone:

```bash
# repository root — builds :local images and syncs the client. Does not start the cluster.
task build
```

If those images already exist:

```bash
cd client
VERSION=local uv run fbenv create
uv run fbenv start
uv run fbenv status
```

`fbull` talks to `localhost:50055` when cluster is `local`. The generated directory is `{{ cookiecutter.project_slug }}`. From that directory:

```bash
fbull backtest create --file backtest.yaml
fbull backtest run demo {{ cookiecutter.project_slug }}/macd.py --yes
fbull backtest run demo {{ cookiecutter.project_slug }}/rsi.py --yes
```

Remote cluster: `fbenv use remote …`, then `fbenv create` and `fbenv start`. Set `FOREVERBULL_BROKER_ADDRESS` or `remote.broker_address`. See the clone's [deploy/README.md](https://github.com/lhjnilsson/foreverbull/blob/main/deploy/README.md).

Do not enable live trading from this template.

## Testing

Tests live in `tests/` and use the `foreverbull-testing` pytest plugin.

Unit tests (Docker Postgres via Testcontainers). They do not use Kind:

```bash
uv run pytest
# or: python -m pytest
```

Live backtest tests against a named broker backtest (needs `fbenv` / Kind running):

```bash
uv run pytest --backtest demo
```
