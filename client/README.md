# Python client workspace

Four packages, one [uv](https://docs.astral.sh/uv/) workspace. Python 3.12+.

| Package | Path | Role |
| --- | --- | --- |
| `foreverbull` | `src/foreverbull/` | Client library: gRPC stubs, algorithm models, NNG workers |
| `foreverbull-cli` | `cli/` | `fbull` (broker) and `fbenv` (Kind/Helm) |
| `foreverbull-testing` | `testing/` | Pytest plugin for algorithm unit tests |
| `foreverbull-zipline` | `zipline/` | Zipline gRPC engine (also the `lhjnilsson/zipline` image) |

`client/src/foreverbull/pb/` is generated protobuf. Do not edit it. Change `proto/` at the repo root and run `task proto` (or `task client:proto:generate`).

## Setup

From this directory:

```bash
uv sync
```

That installs all four workspace members plus dev tools (pytest, cookiecutter, grpcio-tools).

## fbull vs fbenv

Both ship in `foreverbull-cli`. After `uv sync`:

```bash
uv run fbull --help    # backtests against a running broker
uv run fbenv --help    # Kind (local) or Helm (remote) install
```

- **`fbenv`** installs the platform. Local default is a Kind cluster named `foreverbull` and Helm chart `deploy/helm/foreverbull`. It is not a Docker Compose / Docker-network stack.
- **`fbull`** talks to that broker over gRPC (`localhost:50055` when `cluster: local`). It does not start the cluster.

Bring up local images + Kind from the repository root with `task env`, or from here:

```bash
VERSION=local uv run fbenv create
uv run fbenv status
```

Details: [cli/README.md](cli/README.md), [deploy/README.md](../deploy/README.md).

## Tests

Default pytest (this `pyproject.toml`) ignores `tests/end_to_end` and `tests/integration`. Unit tests use Docker Testcontainers for Postgres; they do not use Kind.

```bash
uv run pytest                 # unit tests across the workspace
uv run pytest tests/          # this library
uv run pytest cli/
uv run pytest testing/
uv run pytest zipline/
```

From the repo root: `task client:test`. SQL fixtures (`fb_database`) need a Docker daemon. See [testing/README.md](testing/README.md).

## Strategy projects

Cookiecutter templates under `templates/` are the supported way to start an algorithm:

```bash
uv run cookiecutter templates/simple      # no TA-Lib
uv run cookiecutter templates/advanced    # needs TA-Lib
```

From GitHub (root `cookiecutter.json` picks the same two):

```bash
cookiecutter https://github.com/lhjnilsson/foreverbull.git
```

Generated READMEs show how to run against a Kind/`fbenv` broker. Do not enable live trading from these templates.
