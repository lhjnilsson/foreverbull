foreverbull
===================

Foreverbull is a trading platform that enables continuous delivery of algorithmic trading strategies.
The goal of the project is to provide a platform that enables rapid development and deployment of trading strategies and enable confidence in the results.

At this moment the marketdata is provided by Yahoo together with fetching ETF- holdings directly from iShares API.
Trading is supported through [Alpaca](https://alpaca.markets/) who provides both live and paper trading.

The architecture is deeper describes in the [Architecture Overview](#architecture) section.
However in short the platform consits of several services that are running in containers.
- `Foreverbull`: Main application, orchestrates the other services
- `Minio`: Store ingestions for backtesting as well as backtest execution results in raw format
- `PostgreSQL`: Persistent storage for application data
- `NATS`: Message queue for communcation between services in the main application
- `Grafana(Including LOKI)`: For monitoring of the platform as well as visualization of backtest results


Below you will find a quickstart guide to get started with the project.

# Getting Started

## Requirements

- Docker
- Python 3.12+
- Cookiecutter

## Quickstart

```bash
cookiecutter https://github.com/lhjnilsson/foreverbull.git
```

Cookiecutter will guide you through the steps for setting up a new project.
There are two starter templates to choose from, one of which requires you to have talib installed.

# Architecture

![Overview](assets/diagram/overview.png)


## Application Domains

![Domains](assets/diagram/domains.png)

- `Backtest`: Backtesting of trading strategies using zipline
- `Finance`: Ingesting marketdata as well as live trading
- `Service`: Responsible for orchestrating alhorithmic trading strategies in containers
- `Strategy`: Scheduling of backtesting as well as live trading

# Development and Contributing

The project is prodominantly written in Go where the domains reside under `pkg` together with internal parts in `internal`. CLI entry points reside in `cmd`.
Python client libraries reside in `client` and consists of four parts
- `foreverbull`: Main client application that connects to foreverbull-broker
- `foreverbull-zipline`: Wrapper around zipline with gRPC Communication
- `foreverbull-cli`: CLI to manage backtesting, trading as well as environment
- `foreverbull-testing`: Custom Pytest fixtures to simplify testing

For go specific information see the [Go README](https://github.com/lhjnilsson/foreverbull/blob/main/go/README.md) inside the `pkg`- folder.
For python specific information see the [Python README](https://github.com/lhjnilsson/foreverbull/blob/main/python/README.md) inside the `client`- folder.

