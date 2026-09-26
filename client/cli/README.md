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
uv run fbull backtest run demo path/to/strategy.py --yes
uv run fbull backtest run demo path/to/strategy.py --start 2021-01-01 --end 2021-03-31 --dry
uv run fbull backtest executions --backtest demo
uv run fbull backtest execution EXECUTION_ID
uv run fbull backtest delete demo
```

`create` stores the universe, downloads daily bars, and ingests them into Zipline (progress: Creating, Downloading, Ingesting, Storing). A symbol whose download fails is stored with `enabled: false`.

The broker environment variable `MARKET_DATA_PROVIDER` selects the price source. Set it before create. Bars already stored for a symbol and day are kept.

| Provider | Broker setting | What it prices |
| --- | --- | --- |
| Yahoo (default) | unset, or `yahoo` | Asset lookup and daily bars |
| Stooq | `stooq` | Daily bars from Stooq. Asset lookup stays on Yahoo |

Both providers can ingest the same Nasdaq-100 file, [examples/nasdaq100.yaml](../../examples/nasdaq100.yaml) (name `nasdaq100`, benchmark `QQQ`, 101 constituents, 2023-01-01 through 2023-03-31). From `client/`:

```bash
# Yahoo (default after fbenv create)
uv run fbull backtest create --file ../examples/nasdaq100.yaml
uv run fbull backtest get nasdaq100

# Stooq on the local Kind cluster (context kind-foreverbull)
kubectl --context kind-foreverbull -n foreverbull set env deployment/foreverbull MARKET_DATA_PROVIDER=stooq
kubectl --context kind-foreverbull -n foreverbull rollout status deployment/foreverbull
uv run fbull backtest create --file ../examples/nasdaq100.yaml
```

`fbenv create` and `fbenv update` reinstall the chart and leave `MARKET_DATA_PROVIDER` unset, which selects Yahoo. Set `stooq` again after those commands when you still want Stooq bars. Use an empty database for the Stooq create when Yahoo bars for these symbols are already stored (`fbenv stop --remove --purge` on local deletes the Kind cluster).

`--file` YAML matches the cookiecutter `backtest.yaml` shape (`name`, `benchmark`, `periods` with `start_date` / `end_date` / `assets`). Without `--file`, create prompts for a name, an ETF, and dates. `fbull backtest etfs` prints the same funds the prompt offers (name, benchmark, inception). The known fund is `iShares S&P 100` (benchmark `OEF`, inception 2000-10-23). An unknown name stops before the broker is called. The backtest is point-in-time iShares holdings.

`run` starts a backtest session on the broker and executes the strategy file on the host (NNG ports `27000`–`27015` on local Kind). The backtest must already exist from `create`. `--yes` skips the confirmation prompt. After a run, `fbull` prints a Grafana link at `http://localhost:3000` (local Kind host port).

### Trading

```bash
uv run fbull trading run demo path/to/strategy.py --dry
```

This talks to a running broker. Alpaca live/paper trading stays off unless the broker has `TRADING_ENABLED=true`. Do not enable live trading from this README. Do not put Alpaca keys in the Helm chart.

## fbenv

Installs the six workloads (Postgres, NATS JetStream, Minio, Zipline, broker, Grafana) with **one Helm chart**. It does **not** create a Docker network or mount `/var/run/docker.sock`.

- `local` (default): Kind cluster named `foreverbull`, chart in namespace `foreverbull`, `expose.hostPort=true`. Host ports match Kind `extraPortMappings` (`5432`, `4222`, `9000`, `50055`, `27000`–`27015`, `3000`).
- `remote`: `helm upgrade --install` against `--kubeconfig` / `--context` / `--namespace`. Requires `--broker-address` (or `remote.broker_address` / `FOREVERBULL_BROKER_ADDRESS`).

```bash
uv run fbenv status
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

`--cluster`, `--kubeconfig`, `--context`, `--namespace`, `--broker-address` override the selected cluster for one command.

`VERSION` is the image tag (`lhjnilsson/foreverbull|zipline|fb-grafana:{VERSION}`). Use `local` with images built via `task build` / `task build:docker`. `task build` does not create the cluster.

Cluster config files and Helm recipes: [deploy/README.md](../../deploy/README.md).

## Tests

From `client/`:

```bash
uv run pytest cli/
```
