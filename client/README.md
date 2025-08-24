# Foreverbull/Client

Client libraries are a collection of python packages intended to communicate with the Foreverbull platform.
There are total of four packages, out of which one is Backtest-Engine(Zipline)

## Packages
### src
src contains the core client modules, used to communicate with the Foreverbull platform as well as execute algorithms.

### cli
cli houses command line interface(s) for interacting with the Foreverbull platform over cli.
It has two distinct cli implementations, one for focused on communication with the platform as well as executing algorithms; both backtest and live trading. The other cli is targeted towards environment and making sure the platform is operational.

### testing
testing has pytest fixtures and plugins to simplify unit testing of algorithms prior to running full backtests.

### zipline
Zipline is wrapper around the Zipline library to provide a gRPC interface for communication with the Foreverbull platform.

## Setup
All python packages are managed by uv.
If running them from source its recommended to install uv prior, to simplify the setup.

For full documentation regarding uv please visit the uv website.

### Quickstart

In simplest form, based on that you have uv installed, you can execute tests by running:
```bash
uv run pytest [path/to/test.py]
```