# Python client workspace

Four packages, one [uv](https://docs.astral.sh/uv/) workspace. Python 3.12+.

| Package | Path | Role |
| --- | --- | --- |
| `foreverbull` | `src/foreverbull/` | Client library: gRPC stubs, algorithm models, NNG workers |
| `foreverbull-cli` | `cli/` | `fbull` (broker) and `fbenv` (Kind/Helm) |
| `foreverbull-testing` | `testing/` | Pytest plugin for algorithm unit tests |
| `foreverbull-zipline` | `zipline/` | Zipline gRPC engine (also the `lhjnilsson/zipline` image) |

📝 `client/src/foreverbull/pb/` is generated protobuf (`*_pb.py` and `*_pb_grpc.py`). Do not edit it. Change `proto/` at the repo root and run `task proto` (or `task client:proto:generate`). The wheel ships those modules, so `uv sync` does not run buf.

## Setup

From this directory:

```bash
uv sync
```

That installs the client library, dev tools (pytest, cookiecutter, buf), and `foreverbull-cli` (`fbull` and `fbenv`).

## fbull vs fbenv

Both ship in `foreverbull-cli`. After `uv sync`:

```bash
uv run fbull --help    # backtests against a running broker
uv run fbenv --help    # Kind (local) or Helm (remote) install
```

- **`fbenv`** installs the platform. Local default is a Kind cluster named `foreverbull` and Helm chart `deploy/helm/foreverbull`. It is not a Docker Compose / Docker-network stack.
- **`fbull`** talks to that broker over gRPC (`localhost:50055` when `cluster: local`). It does not start the cluster.

From the repository root, `task build` builds the `:local` images and syncs this workspace. It does not start the cluster. Then:

```bash
VERSION=local uv run fbenv create
uv run fbenv start
uv run fbenv status
```

`create` installs the release with the workloads stopped. `start` scales them to 1.

`uv run fbenv` is the Kind/Helm installer. A `fbenv` already on `PATH` can be an older program that starts Docker containers directly.

`fbull backtest create` downloads daily bars and ingests them. Yahoo is the default price source (no API key). `--provider` overrides an optional YAML `provider` field. The [root quickstart](../README.md#ingest-data-and-create-a-backtest) creates the demo universe with Yahoo by default, or Stooq / Alpaca when selected.

Details: [cli/README.md](cli/README.md), [deploy/README.md](../deploy/README.md).

## Tests

Default pytest (this `pyproject.toml`) ignores `tests/end_to_end` and `tests/integration`. Unit tests use Docker Testcontainers for Postgres; they do not use Kind. The Kind demo is the root suite `tests/end_to_end/`, run with `task test:integration`.

```bash
uv run pytest                 # unit tests across the workspace
uv run pytest tests/          # this library
uv run pytest cli/
uv run pytest testing/
uv run pytest zipline/
```

From the repo root: `task client:test`. SQL fixtures (`fb_database`) need a Docker daemon. See [testing/README.md](testing/README.md).

## Strategy projects

Cookiecutter templates under `templates/` are the supported way to start an algorithm.
Generated strategies size from `portfolio_value` (equal-weight deployable capital,
cash reserve, max positions, rebalance every bar). See [templates/README.md](templates/README.md).

```bash
uv run cookiecutter templates/simple      # no TA-Lib
uv run cookiecutter templates/advanced    # needs TA-Lib
```

The generated directory is the project slug. From `client/`, the strategy file is `simple_foreverbull/crossover.py` (or `advanced_foreverbull/macd.py`).

From GitHub (root `cookiecutter.json` picks the same two):

```bash
cookiecutter https://github.com/lhjnilsson/foreverbull.git
```

Generated READMEs show how to run against a Kind/`fbenv` broker. Do not enable live trading from these templates.
