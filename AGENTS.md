# Agent notes

Foreverbull is a Go trading platform with a Python client workspace. Work from the repository root unless a command says otherwise. Python is managed with **uv** (workspace in `client/`). Do not use rye.

Install [Task](https://taskfile.dev/) (`go-task`) to run the root `taskfile.yaml` (it includes `client/taskfile.yaml` and `grafana/taskfile.yaml`).

## Layout

| Path | What it is |
| --- | --- |
| `cmd/server/` | Go broker/server entrypoint |
| `pkg/` | Go domains: `backtest`, `finance`, `service`, `strategy` |
| `pkg/pb/` | Generated Go protobuf (plus a few hand-written helpers) |
| `internal/` | Shared Go internals (env, grpc, storage, stream, Kubernetes container runtime, testcontainers helpers) |
| `proto/` | Protobuf **sources**. Edit these, then regenerate. |
| `client/` | uv workspace: `foreverbull`, `foreverbull-cli`, `foreverbull-testing`, `foreverbull-zipline` |
| `client/src/foreverbull/` | Python client library |
| `client/cli/` | `fbull` and `fbenv` CLIs (`fbenv` is Kind/Helm, not a Docker network) |
| `client/zipline/` | Zipline gRPC engine + Docker image |
| `client/testing/` | Pytest plugin/fixtures (`fb_database`, etc.) |
| `deploy/` | Kind config, Helm chart, cluster-config example |
| `docker/Dockerfile` | Local `lhjnilsson/foreverbull` image |
| `grafana/` | Grafana image + datasource plugin |
| `internal/container/` | Kubernetes runtime for extra strategy-worker pods (no docker.sock) |

## Prerequisites

- Docker (unit tests use Testcontainers; Kind also needs a Docker daemon; integration/e2e need local images)
- Kind, Helm, and kubectl (local `fbenv` / Kind cluster)
- Go 1.24 (see `go.mod`)
- Python 3.12+ and [uv](https://docs.astral.sh/uv/)
- Task
- For proto generation: `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, and `uv sync` in `client/` (pulls `grpcio-tools`)

```bash
cd client && uv sync
```

## Testing

Many Go and Python unit tests start Postgres/NATS/Minio via Testcontainers. Have Docker running. They are not live-trading tests. **Do not convert unit/component tests to Kind.** Testcontainers stay on Docker even though `fbenv` installs the platform with Kind/Helm.

### Go

Domain unit tests (no `integration` build tag):

```bash
task test:unit
# equivalent per domain:
task backtest:test:unit
task finance:test:unit
task service:test:unit
task strategy:test:unit
```

Those run `go test ./...` inside `pkg/backtest`, `pkg/finance`, `pkg/service`, and `pkg/strategy`. Extra CLI args pass through as `task backtest:test:unit -- -count=1`.

From the repo root you can also run:

```bash
go test ./pkg/... ./internal/...
```

Backtest **integration** tests are tagged `//go:build integration` (`pkg/backtest/module_test.go`, `pkg/backtest/stream/backtest_test.go`). They need the local Zipline image first:

```bash
task backtest:test:integration
```

That depends on `client:docker:zipline`, then runs `go test . -tags=integration` and `go test ./stream/ -tags=integration` in `pkg/backtest`.

Plain `go test ./...` does **not** run those files unless you pass `-tags=integration`.

### Python

All Python tests run from `client/` with uv. `client/pyproject.toml` default addopts ignore `tests/end_to_end` and `tests/integration`.

```bash
cd client
uv sync
uv run pytest                 # unit tests across the workspace
uv run pytest tests/          # client library tests
uv run pytest zipline/        # foreverbull-zipline
uv run pytest cli/            # fbull / fbenv
uv run pytest testing/        # pytest plugin
```

From the repo root:

```bash
task client:test
task client:test -- tests/
task client:test -- zipline/
task client:test -- cli/
task client:test -- testing/
```

The `foreverbull-testing` package is a pytest plugin (`fb_database` and related fixtures). Several client and zipline tests expect it to be installed via `uv sync`.

### Docker / Zipline images

```bash
task client:docker:zipline    # build lhjnilsson/zipline:local (also builds the foreverbull wheel)
task build:docker             # build lhjnilsson/foreverbull:local
task grafana:build:docker     # build lhjnilsson/fb-grafana:local
```

`task client:docker:zipline` is the Zipline image used by Go backtest integration tests.

### Python integration / end-to-end

These are **not** part of default `uv run pytest`:

```bash
task test:integration
```

That builds Foreverbull, Zipline, and Grafana local images, then from `client/` runs:

```bash
VERSION=local ECHO=true uv run pytest tests/end_to_end/test_demo.py -vs
```

`client/tests/integration/` is a heavier Zipline/ingest suite; pass that path explicitly if you need it. Some fixtures skip unless `BACKTEST_IMAGE` is set.

## Protobuf

Sources live in `proto/`. Generated code:

- Go: `pkg/pb/**` (`*.pb.go`, `*_grpc.pb.go`) and `internal/stream/pb/`
- Python: `client/src/foreverbull/pb/` (`*_pb2.py`, `*_pb2.pyi`, `*_pb2_grpc.py`)

**Do not hand-edit generated protobuf.** Change `.proto` files and regenerate:

```bash
task proto                    # Go + Python
task proto:generate           # Go only
task client:proto:generate    # Python only (rewrites imports after protoc)
task proto:clean              # deletes generated *_pb2* and *.pb.go
```

Hand-written helpers next to generated code (safe to edit):

- `pkg/pb/timestamp.go`
- `client/src/foreverbull/pb/pb_utils.py`

`mock_*.go` files are from `task mock:generate` (mockery), not protoc.

## Local / dev environment

The platform install is **Kind + Helm**, not a Docker Compose / Docker-network stack. `fbenv` creates a Kind cluster named `foreverbull` (default), then `helm upgrade --install`s `deploy/helm/foreverbull` into namespace `foreverbull`. The broker pod does **not** mount `/var/run/docker.sock`. Extra strategy-worker pods are created with the in-cluster Kubernetes API (ServiceAccount RBAC). Do not add a docker.sock volume to the broker, and do not talk to the Docker API from the broker runtime.

Bring up the full stack (local images + `fbenv`):

```bash
task env
```

That builds Zipline, Grafana, and Foreverbull `:local` images, `uv sync`s the client, then from `client/`:

```bash
uv run fbenv stop --remove
VERSION=local uv run fbenv create
```

Useful `fbenv` commands (from `client/`):

```bash
uv run fbenv status
uv run fbenv use                         # print selected cluster
uv run fbenv use local                   # Kind (default); writes .foreverbull/config.yaml
uv run fbenv use remote --kubeconfig … --context … --namespace … --broker-address host:50055
VERSION=local uv run fbenv create        # Kind cluster + helm upgrade --install (default start)
uv run fbenv start                       # scale deployments to 1
uv run fbenv stop                        # scale to 0
uv run fbenv stop --remove               # helm uninstall
uv run fbenv stop --remove --purge       # uninstall; local also deletes the Kind cluster; deletes .foreverbull data except config.yaml
uv run fbenv update                      # reload images (local) and helm upgrade --reuse-values
```

CLI flags `--cluster`, `--kubeconfig`, `--context`, `--namespace`, `--broker-address` override the selected cluster for one command.

Published host ports (Kind `extraPortMappings` + chart `expose.hostPort=true`): Postgres `5432`, NATS `4222`, Minio `9000`, Foreverbull gRPC `50055`, backtest `27000–27015`, Grafana `3000`. Persistence is cluster PVCs (`persistence.*`), not host binds under `.foreverbull/{postgres,minio,nats}`.

Python broker stubs resolve the address in this order: `FOREVERBULL_BROKER_ADDRESS` → `remote.broker_address` when `cluster: remote` → `localhost:50055` when `cluster: local`. Grafana in-cluster uses `BROKER_URL=foreverbull:50055`.

### Cluster config

Users pick `cluster: local` (default) or `remote`. Canonical keys: `deploy/cluster-config.example.yaml`.

| File / env | Role |
| --- | --- |
| `.foreverbull/config.yaml` | Project config (cwd; do not commit) |
| `~/.foreverbull/config.yaml` | Optional user override |
| `FOREVERBULL_CONFIG` | Explicit path; wins over the files above |
| `FOREVERBULL_BROKER_ADDRESS` | Runtime broker `host:port`; highest priority for stubs/CLI |

Merge order (later wins): built-in defaults → user → project → `FOREVERBULL_CONFIG`. Remote requires `remote.broker_address` (or `--broker-address` / `FOREVERBULL_BROKER_ADDRESS`). Local always uses `localhost:50055` unless the env var is set.

### Helm chart checks (no cluster)

```bash
helm lint deploy/helm/foreverbull
bash deploy/helm/check-chart.sh
# equivalent: helm template … and assert RBAC exists and docker.sock is not mounted
```

Manual Kind/Helm (same as `fbenv create` local): `task kind:create`, `task kind:load`, `task helm:install`. Recipes: `deploy/README.md`.

### Environment variables

Go server defaults are in `internal/environment/env.go`. Empty vars are filled at process start.

| Variable | Default / notes |
| --- | --- |
| `GRPC_PORT` | `50055` |
| `HTTP_PORT` | `8080` |
| `SERVER_ADDRESS` | `foreverbull` (in-cluster Service DNS other pods use) |
| `ZIPLINE_CONTAINER_NAME` | Go default `foreverbull_zipline`. **Helm** sets `zipline` (Service DNS). `internal/container` looks up that name as a Pod or `app.kubernetes.io/name` |
| `BACKTEST_PORT_RANGE_START` / `END` | `27000` / `27015` |
| `LOG_LEVEL` | `warning` (`debug`/`info`/`warning`/`error`); local `fbenv` sets `debug` on the broker (`--set broker.logLevel=debug`) |
| `POSTGRES_URL` | Go default: `postgres://postgres:foreverbull@localhost:5432/postgres?sslmode=disable`. **Helm** Postgres is user/password/db `foreverbull`, and the broker pod gets `postgres://foreverbull:foreverbull@postgres:5432/foreverbull`. Host-side against local Kind: `postgres://foreverbull:foreverbull@localhost:5432/foreverbull?sslmode=disable` |
| `NATS_URL` | `nats://localhost:4222` (in-cluster: `nats://nats:4222`) |
| `NATS_DURABLE` | `foreverbull` |
| `NATS_DELIVERY_POLICY` | `all` |
| `MINIO_URL` | `localhost:9000` (in-cluster: `minio:9000`) |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET` | `minioadmin` / `minioadmin` |
| `POD_NAMESPACE` / `KUBERNETES_NAMESPACE` | Broker chart sets `POD_NAMESPACE` from the pod namespace; `internal/container` uses it for the in-cluster client |
| `MARKET_DATA_PROVIDER` | `yahoo` |
| `TRADING_ENABLED` | `false` — do not enable live trading unless explicitly requested; never put real Alpaca keys in the chart |
| `TRADING_PROVIDER` | `alpaca` |
| `ALPACA_MARKETS_BASE_URL` | `https://paper-api.alpaca.markets` |
| `ALPACA_MARKETS_API_KEY` / `ALPACA_MARKETS_API_SECRET` | unset; required only if trading is enabled |
| `VERSION` | Image tag for `fbenv` / chart `global.imageTag` (`lhjnilsson/foreverbull`, `lhjnilsson/zipline`, `lhjnilsson/fb-grafana`). Use `local` with `task env` / locally built images; default `latest` |
| `FOREVERBULL_BROKER_ADDRESS` | Python stubs/CLI broker `host:port` (highest priority) |
| `FOREVERBULL_CONFIG` | Explicit cluster config YAML path |
| `DOCKER_NETWORK` | Legacy default `foreverbull`. Used by Testcontainers helpers, **not** by `fbenv` or the broker |
| `DATABASE_URL` | Zipline/worker Postgres URL; pytest plugin sets this via Testcontainers |
| `BROKER_HOSTNAME` | Worker/NNG hostname (default `127.0.0.1`; in-cluster workers get `SERVER_ADDRESS`) |
| `NAMESPACE_PORT` | Required for the worker namespace socket |
| `LOGGING_LEVEL` | Python client/CLI logging (`fbull -v/-vv/-vvv`) |
| `ECHO` | If set, CLI tests print stdout |
| `BACKTEST_IMAGE` / `BROKER_IMAGE` | Override images in some Python fixtures |

Python Minio client (`Storage.from_environment`) also reads `STORAGE_ENDPOINT` (default `localhost:9000`), `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_SECURE`.

Package docs: [client/README.md](client/README.md), [client/cli/README.md](client/cli/README.md), [client/zipline/README.md](client/zipline/README.md), [client/testing/README.md](client/testing/README.md), [deploy/README.md](deploy/README.md). Cookiecutter templates: `client/templates/simple` and `client/templates/advanced` (root `cookiecutter.json`).

## CLI and fbenv

Both live in `client/cli/` (`foreverbull-cli`). Entry points in `client/cli/pyproject.toml`:

- `fbull` → `foreverbull_cli.bull.cli:cli` — backtests and trading against a running broker (`fbull backtest …`, `fbull trading …`)
- `fbenv` → `foreverbull_cli.env.cli:cli` — Kind/Helm environment (`local` Kind cluster or `remote` kubeconfig)

After `uv sync` in `client/`:

```bash
uv run fbull --help
uv run fbenv --help
```

## Python lint

```bash
task client:lint              # uvx ruff check
task client:format            # uvx ruff format
```

Ruff ignores generated `client/src/foreverbull/pb/` and cookiecutter `templates/`.
