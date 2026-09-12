# {{ cookiecutter.project_slug }}

Generated from the Foreverbull **simple** cookiecutter template (no TA-Lib).
The sample algorithm buys when a short moving average crosses above a long one.

Cookiecutter plus `foreverbull-testing` is the supported way to write algorithms.
Do not copy a deleted `example_algorithms` tree.

# Installation

Skip this section if you already installed dependencies during generation.

Use a virtual environment, then install from `requirements.txt` (Git checkouts of
`foreverbull`, `foreverbull-testing`, and `foreverbull-cli`):

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

If you have [uv](https://docs.astral.sh/uv/) and this repo checked out locally,
you can also `uv sync` in `client/` and run tools from there.

# Usage

Bring up a local Foreverbull environment (`fbenv create`), then:

```bash
fbull backtest create --file backtest.yaml
fbull backtest run demo {{ cookiecutter.project_slug }}/crossover.py --yes
```

`backtest.yaml` is a small demo universe. Change dates and symbols as needed.

# Testing

Tests live in `tests/` and use the `foreverbull-testing` pytest plugin
(`asset_manager`, `portfolio_manager`, and optionally `backtest`).

Unit tests (Docker Postgres via Testcontainers; prices from yfinance):

```bash
python -m pytest
```

Live backtest tests against a named broker backtest:

```bash
python -m pytest --backtest demo
```
