# Foreverbull datasource

Grafana backend datasource for Foreverbull backtest executions.

Plugin ID: `lhjnilsson-foreverbull-datasource`. Requires Grafana 10.4 or later.

The backend talks to the Foreverbull gRPC broker (`BROKER_URL`, in-cluster default `foreverbull:50055`). Query type `GetExecutionMetric` loads one execution by ID and returns selected metrics over time:

- `portfolio_value`
- `alpha`
- `beta`
- `sharpe`
- `sortino`
- `capital_used`
- `long_count`
- `short_count`
- `long_value`
- `short_value`

On a local Kind install the chart publishes Grafana at `http://localhost:3000`. `fbull backtest run` prints a dashboard link with `var-execution` set.

This plugin is built into the `lhjnilsson/fb-grafana` image. Do not enable live trading from Grafana.
