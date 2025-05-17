# simple

This contains generated code for a simple foreverbull project.
The projects contains a simple algorithm that buys stocks with a crossover strategy.

# Installation

The section below assumes that you did not complete the run_setup hook when creating the project.
If you did, you can skip this section.

It is recommended to use a virtual environment for this project.
To do so, run the following commands:
```bash
python -m venv .venv
source .venv/bin/activate
```

Then install the dependencies:
```bash
pip install -r requirements.txt
```

# Usage

To create backtests, run the following command:
```bash
fbull backtest create [symbols.json]
```

This will both create the backtest as well as ingest financial data to be used during backtesting.

You can also specify a name for the backtest by using `--name`, otherwise it will be named after the file.
```bash
fbull backtest create [symbols.json] --name [backtest_name]
```

You can now run the backtest by running the following command:
```bash
fbull backtest run [backtest_name] simple/crossover.py
```

# Testing

Tests are located in the `tests` directory.
There are two types of tests:
- Unit tests, which test the individual functions in the code
- Backtest tests, which test the backtesting functionality and the performance of the algorithm

To run unit tests, run the following command:
```bash
python -m pytest
````

To run with backtesting, run with the `--backtest` flag and specify the backtest name:
```bash
python -m pytest --backtest [backtest_name]
````

# Github Actions

The project comes with Github Actions that will run the tests on every push.
