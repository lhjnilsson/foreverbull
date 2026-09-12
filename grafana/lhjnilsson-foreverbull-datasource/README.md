# Foreverbull Grafana datasource

Backend Grafana datasource for Foreverbull backtest executions and metrics (`lhjnilsson-foreverbull-datasource`).

Plugin ID: `lhjnilsson-foreverbull-datasource`. Grafana 10.4+.

The Go backend (`pkg/`) calls the Foreverbull gRPC broker. The React UI (`src/`) is a query editor: pick an execution ID and one or more metrics.

The image Helm installs is `lhjnilsson/fb-grafana` (`grafana/Dockerfile` in this repo). In-cluster it sets `BROKER_URL=foreverbull:50055`. Local Kind publishes Grafana at `http://localhost:3000`.

## Prerequisites

- Node.js 22+ (see `engines` in `package.json`) and npm
- Go 1.23+
- [Mage](https://magefile.org/): `go install github.com/magefile/mage@latest`

For the platform Grafana pod: Docker, Kind, Helm, and the rest of the Foreverbull stack (`task env` / `fbenv`).

## Layout

| Path | What it is |
| --- | --- |
| `src/` | React / TypeScript UI (config editor, query editor) |
| `pkg/` | Backend datasource (gRPC client, `/executions` and `/metrics` resources) |
| `provisioning/` | Datasource and dashboard YAML baked into `lhjnilsson/fb-grafana` |
| `docker-compose.yaml` | Isolated Grafana 11 for plugin UI work (see below) |
| `Magefile.go` | Cross-compiles `gpx_foreverbull_*` into `dist/` |

The backend reads `BROKER_URL`. Helm sets `foreverbull:50055`. If unset, the Go code defaults to `localhost:50055`.

## Build

From this directory:

```bash
npm install
npm run build    # frontend → dist/
mage -v          # backend binaries → dist/gpx_foreverbull_*
```

Watch the frontend:

```bash
npm run dev
```

Rebuild Go after changes under `pkg/`:

```bash
mage -v
```

Platform image (from `grafana/`):

```bash
task build:docker    # lhjnilsson/fb-grafana:local
```

Then load it with `fbenv` / `task env`. Signing is not required for local Kind.

## Queries

Query type `GetExecutionMetric` loads a backtest execution by ID. Metrics: `portfolio_value`, `alpha`, `beta`, `sharpe`, `sortino`, `capital_used`, `long_count`, `short_count`, `long_value`, `short_value`.

`fbull backtest run` prints a dashboard link at `http://localhost:3000` on local Kind.

## Plugin-only Grafana (compose)

`docker-compose.yaml` runs Grafana 11 on host port `3001` with this unsigned plugin mounted from `dist/`. It joins an **external** Docker network named `foreverbull`. Kind/`fbenv` does **not** create that network. Use compose only when you already have a broker reachable as `foreverbull:50055` on that Docker network. For the product Grafana, use the Helm chart (`lhjnilsson/fb-grafana`) and `http://localhost:3000`.

```bash
npm run server    # docker compose up --build
```

Unsigned loading: `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=lhjnilsson-foreverbull-datasource`.

## Tests

```bash
npm run test:ci      # Jest
npm run lint
npm run typecheck
go test ./pkg/...    # backend unit tests
```

Playwright (`npm run e2e`) expects `npm run server` and a reachable broker.
