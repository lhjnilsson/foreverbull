# foreverbull-cli

Python CLIs for a running Foreverbull broker and for installing the platform.

After `uv sync` in `client/`:

```bash
uv run fbull --help
uv run fbenv --help
```

Entry points (`client/cli/pyproject.toml`):

| Command | Module | Role |
| --- | --- | --- |
| `fbull` | `foreverbull_cli.bull.cli` | Backtests (and trading) against a running broker |
| `fbenv` | `foreverbull_cli.env.cli` | Kind/Helm environment (`local` or `remote`) |

Requires Python 3.12+. Local `fbenv` also needs Docker, Kind, Helm, and kubectl.

`fbull` does not start the cluster. `fbenv` does not run algorithms. Use both: `fbenv create`, `fbenv start`, then `fbull backtest …`.

## fbull

Talks to the broker over gRPC. Subcommands: `fbull backtest …`, `fbull trading …`.

Broker address (`foreverbull.broker.config.resolve_broker_address`):

1. `FOREVERBULL_BROKER_ADDRESS` (`host:port`)
2. `remote.broker_address` when cluster config is `remote`
3. `localhost:50055` when cluster is `local` (the default)

Logging: `-v` warning, `-vv` info, `-vvv` debug (`LOGGING_LEVEL`).

### Backtests

```bash
uv run fbull backtest list
uv run fbull backtest etfs
uv run fbull backtest create --file backtest.yaml
uv run fbull backtest get demo
uv run fbull backtest get demo --output yaml
uv run fbull backtest run demo path/to/strategy.py
uv run fbull backtest run demo path/to/strategy.py --start 2021-01-01 --end 2021-03-31 --dry
uv run fbull backtest executions --backtest demo
uv run fbull backtest execution EXECUTION_ID
uv run fbull backtest delete demo
```

`create` stores the universe, downloads daily bars, and ingests them into Zipline (progress: Creating, Downloading, Ingesting, Storing). A symbol whose download fails is stored with `enabled: false`.

`create` reads optional `provider` and `profile` from the YAML. `--provider` overrides the file. Omitting `provider` sends `yahoo`. Create prints `Using provider: <name> (<source>)`, where the source is `default`, `from --provider`, or `from <YAML file name>`, for example `Using provider: yahoo (default)` or `Using provider: stooq (from --provider)`. The printed backtest includes the provider. An unknown name exits non-zero and lists the valid providers (`yahoo`, `stooq`, `alpaca`). `profile` is an Alpaca profile name and is ignored unless `provider` is `alpaca`. Do not put API keys in the YAML.

| Provider | How to select it | What it prices |
| --- | --- | --- |
| Yahoo (default) | omit `provider`, or `yahoo` | Asset lookup and daily bars |
| Stooq | `provider: stooq` or `--provider stooq` | Daily bars from Stooq. Asset lookup stays on Yahoo |
| Alpaca | `provider: alpaca` and optional `profile` | Daily bars with that profile's key. Asset lookup stays on Yahoo |

Bars are stored per provider, so Yahoo and Stooq can both keep a close for the same symbol and day. A run reads only its backtest's provider. The Nasdaq-100 file is [examples/nasdaq100.yaml](../../examples/nasdaq100.yaml) (name `nasdaq100`, benchmark `QQQ`, 101 constituents, 2023-01-01 through 2023-03-31). From `client/`:

```bash
# Yahoo (provider omitted)
uv run fbull backtest create --file ../examples/nasdaq100.yaml
uv run fbull backtest get nasdaq100

# Stooq. Use another backtest name when nasdaq100 already exists.
uv run fbull backtest create --file ../examples/nasdaq100.yaml --provider stooq
```

Stooq may block automated downloads with a bot-check page; if create fails, use Yahoo.

`--file` YAML matches the cookiecutter `backtest.yaml` shape (`name`, `benchmark`, `periods` with `start_date` / `end_date` / `assets`) plus optional `provider` and `profile`. Without `--file`, create prompts for a name, an ETF, and dates, and sends `yahoo` unless `--provider` is set. `fbull backtest etfs` prints the same funds the prompt offers (name, benchmark, inception). The known fund is `iShares S&P 100` (benchmark `OEF`, inception 2000-10-23). An unknown name stops before the broker is called. The backtest is point-in-time iShares holdings.

`run` starts a backtest session on the broker and executes the strategy file on the host (NNG ports `27000`–`27015` on local Kind). The backtest must already exist from `create`. After a run, `fbull` prints a Grafana link at `http://localhost:3000` (local Kind host port).

### Trading

```bash
uv run fbull trading run demo path/to/strategy.py --dry
```

This talks to a running broker. Paper trading stays off unless `TRADING_ENABLED=true`. That flag is a kill switch, not a provider. `fbull strategy deploy --no-dry-run` requires `--backend <profile>` plus `--yes` or `--confirm`. Dry-run may omit `--backend`. `--mode live` is refused.

Credentials live only in the profiles file. In the cluster, `TRADING_PROFILES_FILE` is `/etc/foreverbull/profiles.yaml`, mounted from Secret `foreverbull-trading`, key `profiles.yaml`. On local Kind, `fbenv` applies `.foreverbull/trading-profiles.yaml` as that Secret when the file exists, and does not print it. The committed example is [deploy/trading-profiles.example.yaml](../../deploy/trading-profiles.example.yaml) and its credential fields are empty. Do not put keys in the Helm chart, Postgres, or `.foreverbull/config.yaml`.

## fbenv

Installs the six workloads (Postgres, NATS JetStream, Minio, Zipline, broker, Grafana) with **one Helm chart**. It does **not** create a Docker network or mount `/var/run/docker.sock`.

- `local` (default): Kind cluster named `foreverbull`, chart in namespace `foreverbull`, `expose.hostPort=true`. Host ports match Kind `extraPortMappings` (`5432`, `4222`, `9000`, `50055`, `27000`–`27015`, `3000`).
- `remote`: `helm upgrade --install` against `--kubeconfig` / `--context` / `--namespace`. Requires `--broker-address` (or `remote.broker_address` / `FOREVERBULL_BROKER_ADDRESS`).

```bash
uv run fbenv status                      # host checks, then workloads
uv run fbenv use                         # print selected cluster
uv run fbenv use local                   # writes .foreverbull/config.yaml
uv run fbenv use remote --kubeconfig PATH --context NAME --namespace NS --broker-address host:50055
VERSION=local uv run fbenv create        # Kind + helm upgrade --install, workloads stopped
uv run fbenv create --start              # same install, then start
uv run fbenv start                       # scale deployments to 1 and wait until ready
uv run fbenv stop                        # scale to 0
uv run fbenv stop --remove               # helm uninstall
uv run fbenv stop --remove --purge       # uninstall; local also deletes the Kind cluster
uv run fbenv update                      # reload images (local) and helm upgrade --reuse-values
```

`create` and `start` run the host checks first and print nothing when they pass: tools installed, Docker running, at least 4 GB of memory for Docker, and the Kind host ports free (or already held by the `foreverbull` cluster). The first failed check stops the command before it changes anything, with a fix hint. A remote cluster checks only `helm` and `kubectl`.

`--cluster`, `--kubeconfig`, `--context`, `--namespace`, `--broker-address` override the selected cluster for one command.

`VERSION` is the image tag (`lhjnilsson/foreverbull|zipline|fb-grafana:{VERSION}`). Use `local` with images built via `task build` / `task build:docker`. `task build` does not create the cluster.

Cluster config files and Helm recipes: [deploy/README.md](../../deploy/README.md).

## Tests

From `client/`:

```bash
uv run pytest cli/
```
