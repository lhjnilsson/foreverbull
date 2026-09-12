# {{ cookiecutter.project_slug }}

Generated from the Foreverbull **advanced** cookiecutter template.
Sample strategies use [TA-Lib](https://ta-lib.org/) (`macd.py` and `rsi.py`).

Cookiecutter plus `foreverbull-testing` is the supported way to write algorithms.
Do not copy a deleted `example_algorithms` tree.

Install TA-Lib (the C library and the `TA-Lib` Python package) before running
the sample strategies. If you do not have TA-Lib, use the simple template instead.

# Installation

Skip this section if you already installed dependencies during generation.

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
fbull backtest run demo {{ cookiecutter.project_slug }}/macd.py --yes
fbull backtest run demo {{ cookiecutter.project_slug }}/rsi.py --yes
```

# Testing

Tests live in `tests/` and use the `foreverbull-testing` pytest plugin.

```bash
python -m pytest
python -m pytest --backtest demo
```
