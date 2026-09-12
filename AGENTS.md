# Agent notes

Foreverbull is a Go trading platform with a Python client workspace. Work from the repository root unless a command says otherwise. Python is managed with **uv** (workspace in `client/`). Do not use rye.

Install [Task](https://taskfile.dev/) (`go-task`) to run the root `taskfile.yaml` (it includes `client/taskfile.yaml` and `grafana/taskfile.yaml`).

## Layout

| Path | What it is |
| --- | --- |
| `cmd/server/` | Go broker/server entrypoint |
| `pkg/` | Go domains: `backtest`, `finance`, `service`, `strategy` |
| `pkg/pb/` | Generated Go protobuf (plus a few hand-written helpers) |
| `internal/` | Shared Go internals (env, grpc, storage, stream, testcontainers helpers) |
| `proto/` | Protobuf **sources**. Edit these, then regenerate. |
| `client/` | uv workspace: `foreverbull`, `foreverbull-cli`, `foreverbull-testing`, `foreverbull-zipline` |
| `client/src/foreverbull/` | Python client library |
| `client/cli/` | `fbull` and `fbenv` CLIs |
| `client/zipline/` | Zipline gRPC engine + Docker image |
| `client/testing/` | Pytest plugin/fixtures (`fb_database`, etc.) |
| `docker/Dockerfile` | Local `lhjnilsson/foreverbull` image |
| `grafana/` | Grafana image + datasource plugin |

## Prerequisites

- Docker (unit tests use Testcontainers; integration/e2e need local images)
- Go 1.24 (see `go.mod`)
- Python 3.12+ and [uv](https://docs.astral.sh/uv/)
- Task
- For proto generation: `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, and `uv sync` in `client/` (pulls `grpcio-tools`)

```bash
cd client && uv sync
```

## Testing

Many Go and Python unit tests start Postgres/NATS/Minio via Testcontainers. Have Docker running. They are not live-trading tests.

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

Bring up the full stack (local images + `fbenv`):

```bash
task env
```

That builds Zipline, Grafana, and Foreverbull `:local` images, `uv sync`s the client, then:

```bash
uv run fbenv stop --remove
VERSION=local uv run fbenv create
```

`fbenv` creates Docker network `foreverbull` and containers: Postgres, NATS, Minio, Zipline, Foreverbull, Grafana. State/volumes live in `./.foreverbull/` relative to the working directory (`client/` when using `task env`).

Useful `fbenv` commands (from `client/`):

```bash
uv run fbenv status
VERSION=local uv run fbenv create          # create + start (default)
uv run fbenv start
uv run fbenv stop
uv run fbenv stop --remove
uv run fbenv stop --remove --purge         # also deletes .foreverbull data dirs
uv run fbenv update
```

Published host ports: Postgres `5432`, NATS `4222`, Minio `9000`, Foreverbull gRPC `50055`, backtest `27000–27015`, Grafana `3000`. The Python broker stubs talk to `localhost:50055`.

### Environment variables

Go server defaults are in `internal/environment/env.go`. Empty vars are filled at process start.

| Variable | Default / notes |
| --- | --- |
| `GRPC_PORT` | `50055` |
| `HTTP_PORT` | `8080` |
| `SERVER_ADDRESS` | `foreverbull` (hostname other containers use) |
| `DOCKER_NETWORK` | `foreverbull` |
| `ZIPLINE_CONTAINER_NAME` | `foreverbull_zipline` |
| `BACKTEST_PORT_RANGE_START` / `END` | `27000` / `27015` |
| `LOG_LEVEL` | `warning` (`debug`/`info`/`warning`/`error`); fbenv sets `debug` on the broker container |
| `POSTGRES_URL` | Go default: `postgres://postgres:foreverbull@localhost:5432/postgres?sslmode=disable`. **fbenv** Postgres is user/password/db `foreverbull`, and the broker container gets `postgres://foreverbull:foreverbull@postgres:5432/foreverbull`. Host-side against fbenv: `postgres://foreverbull:foreverbull@localhost:5432/foreverbull?sslmode=disable` |
| `NATS_URL` | `nats://localhost:4222` (in-network: `nats://nats:4222`) |
| `NATS_DURABLE` | `foreverbull` |
| `NATS_DELIVERY_POLICY` | `all` |
| `MINIO_URL` | `localhost:9000` (in-network: `minio:9000`) |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET` | `minioadmin` / `minioadmin` |
| `MARKET_DATA_PROVIDER` | `yahoo` |
| `TRADING_ENABLED` | `false` — do not enable live trading unless explicitly requested |
| `TRADING_PROVIDER` | `alpaca` |
| `ALPACA_MARKETS_BASE_URL` | `https://paper-api.alpaca.markets` |
| `ALPACA_MARKETS_API_KEY` / `ALPACA_MARKETS_API_SECRET` | unset; required only if trading is enabled |
| `VERSION` | Image tag for `fbenv` (`lhjnilsson/foreverbull|zipline|fb-grafana:{VERSION}`). Use `local` with `task env` / locally built images; default `latest` |
| `DOCKER_API_VERSION` | Docker SDK pin used by fbenv (default `1.44`) |
| `DATABASE_URL` | Zipline/worker Postgres URL; pytest plugin sets this via Testcontainers |
| `BROKER_HOSTNAME` | Worker/NNG hostname (default `127.0.0.1`) |
| `NAMESPACE_PORT` | Required for the worker namespace socket |
| `LOGGING_LEVEL` | Python client/CLI logging (`fbull -v/-vv/-vvv`) |
| `ECHO` | If set, CLI tests print stdout |
| `BACKTEST_IMAGE` / `BROKER_IMAGE` | Override images in some Python fixtures |

Python Minio client (`Storage.from_environment`) also reads `STORAGE_ENDPOINT` (default `localhost:9000`), `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_SECURE`.

## CLI and fbenv

Both live in `client/cli/` (`foreverbull-cli`). Entry points in `client/cli/pyproject.toml`:

- `fbull` → `foreverbull_cli.bull.cli:cli` — backtests and trading against a running broker (`fbull backtest …`, `fbull trading …`)
- `fbenv` → `foreverbull_cli.env.cli:cli` — local Docker environment

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
