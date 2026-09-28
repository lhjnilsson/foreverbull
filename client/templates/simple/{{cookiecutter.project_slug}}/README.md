# {{ cookiecutter.project_slug }}

Generated from the Foreverbull **simple** cookiecutter template (no TA-Lib).
The strategy is a long-only pandas SMA crossover: a name is in when the short
window is above the long window, and out otherwise.

It sizes from `portfolio_value`: it leaves a `cash_reserve` (default `0.05`),
caps the book at `max_positions` (default `10`), equal-weight deployable capital
across the names that qualify this bar, rebalances those targets every bar, and
flattens names that leave the set. Lookbacks (`short_window` / `long_window`)
and the risk knobs are `Function` parameters. Do not buy a fixed share lot.

Cookiecutter plus `foreverbull-testing` is the supported way to write algorithms.

## Requirements

- Python 3.12+ and [uv](https://docs.astral.sh/uv/) (preferred; pip works)
- A running Foreverbull broker (Kind locally via `fbenv`, or Helm remote)

## Install

Skip this section if you already installed dependencies during generation.

```bash
uv sync
```

The hook also writes `requirements.txt` (git checkouts of `foreverbull`, `foreverbull-testing`, and `foreverbull-cli`). Without uv:

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
fbull backtest run demo {{ cookiecutter.project_slug }}/crossover.py --yes
```

`backtest.yaml` is a small sample universe. Change dates and symbols as needed.

Remote cluster: `fbenv use remote …`, then `fbenv create` and `fbenv start`. Set `FOREVERBULL_BROKER_ADDRESS` or `remote.broker_address`. See the clone's [deploy/README.md](https://github.com/lhjnilsson/foreverbull/blob/main/deploy/README.md).

Do not enable live trading from this template.

## Testing

Tests live in `tests/` and use the `foreverbull-testing` pytest plugin
(`asset_manager`, `portfolio_manager`, and optionally `backtest`).

Unit tests (Docker Postgres via Testcontainers; prices from yfinance). They do not use Kind:

```bash
uv run pytest
# or: python -m pytest
```

Live backtest tests against a named broker backtest (needs `fbenv` / Kind running):

```bash
uv run pytest --backtest demo
```
