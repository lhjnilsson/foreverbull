# Foreverbull Grafana datasource

Grafana backend datasource plugin for Foreverbull backtest executions and metrics. The frontend lives in `src/`; the Go backend in `pkg/` talks to the Foreverbull gRPC broker.

Plugin ID: `lhjnilsson-foreverbull-datasource`

## Prerequisites

- Node.js 22+ (see `engines` in `package.json`) and npm
- Go 1.23+
- [Mage](https://magefile.org/): `go install github.com/magefile/mage@latest`
- Docker, to run Grafana with the plugin loaded
- A Foreverbull stack on the `foreverbull` Docker network (`fbenv create` from the repository root). Compose joins that **external** network so the plugin can reach the broker.

## Layout

| Path | What it is |
| --- | --- |
| `src/` | React / TypeScript UI (config editor, query editor) |
| `pkg/` | Backend datasource (gRPC client, resource handlers) |
| `provisioning/` | Datasource and dashboard YAML used by docker compose |
| `docker-compose.yaml` | Grafana 11 with this unsigned plugin mounted from `dist/` |
| `Magefile.go` | Cross-compiles `gpx_foreverbull_*` into `dist/` |

The backend reads `BROKER_URL` (compose sets `foreverbull:50055`; without it the plugin defaults to `localhost:50555`).

## Build

From this directory:

```bash
npm install
npm run build    # frontend → dist/
mage -v          # backend binaries → dist/gpx_foreverbull_*
```

Watch the frontend while iterating:

```bash
npm run dev
```

Rebuild Go after changes under `pkg/`:

```bash
mage -v
```

## Run Grafana with the plugin

`docker-compose.yaml` expects the `foreverbull` network to already exist:

```bash
# repository root — local images + fbenv stack
task build:docker
task client:docker:zipline
task grafana:build:docker
VERSION=local fbenv create

# this directory
npm run server    # docker compose up --build
```

Grafana is at [http://localhost:3001](http://localhost:3001) with anonymous admin. Unsigned loading is enabled via `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=lhjnilsson-foreverbull-datasource`.

The image `fbenv` uses is built from `grafana/Dockerfile`:

```bash
# from grafana/
task build:docker    # lhjnilsson/fb-grafana:local
```

Signing is not required for local development.

## Tests

```bash
npm run test:ci      # Jest
npm run lint
npm run typecheck
go test ./pkg/...    # backend unit tests
```

Playwright (`npm run e2e`) expects `npm run server` and a reachable broker.

## Queries

Query type `GetExecutionMetric` loads a backtest execution by ID. Metrics include `portfolio_value`, `alpha`, `beta`, `sharpe`, `sortino`, `capital_used`, long/short counts, and long/short values.
