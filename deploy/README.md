# Cluster install (Kind + Helm)

One Helm chart (`deploy/helm/foreverbull`) installs the six platform workloads.
`fbenv` is the CLI installer; you can also `helm upgrade --install` by hand.

- Local default: Kind cluster `foreverbull`, namespace `foreverbull`, chart
  installed into it (`expose.hostPort=true`).
- Remote: `helm upgrade --install` against a user kubeconfig/context/namespace
  (`expose.hostPort=false`).
- Broker does **not** mount `/var/run/docker.sock`. Extra strategy-worker pods
  use in-cluster Kubernetes (ServiceAccount RBAC).
- Go/Python Testcontainers stay on Docker. Do **not** convert unit/component
  tests to Kind.

Chart details: [helm/foreverbull/README.md](helm/foreverbull/README.md).
CLI: [client/cli/README.md](../client/cli/README.md).

## Six workloads

In-cluster DNS names match the Service object names.

| Workload | K8s Service DNS | Image (default) | Host port (local Kind) |
| --- | --- | --- | --- |
| Postgres | `postgres` | `postgres:13.3-alpine` | `5432` |
| NATS JetStream | `nats` | `nats:2.10-alpine` | `4222` |
| Minio | `minio` | `pgsty/silo:RELEASE.2026-09-16T00-00-00Z` | `9000` |
| Zipline | `zipline` | `lhjnilsson/zipline:{tag}` | none (in-cluster gRPC `:50055`) |
| Foreverbull broker | `foreverbull` | `lhjnilsson/foreverbull:{tag}` | `50055`, `27000`–`27015` |
| Grafana | `grafana` | `lhjnilsson/fb-grafana:{tag}` | `3000` |

`{tag}` is `images.*.tag` or `global.imageTag` (`VERSION` in fbenv: `latest` or `local`).

### Host ports (local Kind)

Published on the host when `expose.hostPort=true` (local CLI):

| Port | Service |
| --- | --- |
| `5432` | Postgres |
| `4222` | NATS |
| `9000` | Minio |
| `50055` | Foreverbull gRPC (Python stubs: `localhost:50055`) |
| `27000`–`27015` | Backtest NNG (`BACKTEST_PORT_RANGE_START` / `END`) |
| `3000` | Grafana |

`deploy/kind/cluster.yaml` maps each of those 1:1 onto the Kind control-plane
node (`hostPort` == `containerPort`). The chart sets matching container
`hostPort` when `expose.hostPort=true`. NodePort is not used: these ports are
outside `30000`–`32767`.

Remote installs leave `expose.hostPort=false` (ClusterIP only). Point the CLI
and Python stubs at `remote.broker_address` or `FOREVERBULL_BROKER_ADDRESS`.

### In-cluster env (broker)

Matches `internal/environment/env.go` plus chart overrides:

| Variable | Value |
| --- | --- |
| `POSTGRES_URL` | `postgres://foreverbull:foreverbull@postgres:5432/foreverbull` |
| `NATS_URL` | `nats://nats:4222` |
| `MINIO_URL` | `minio:9000` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET` | `minioadmin` / `minioadmin` |
| `SERVER_ADDRESS` | `foreverbull` |
| `ZIPLINE_CONTAINER_NAME` | `zipline` |
| `POD_NAMESPACE` | pod namespace (fieldRef) |
| `GRPC_PORT` | `50055` |
| `BACKTEST_PORT_RANGE_START` / `END` | `27000` / `27015` |
| `LOG_LEVEL` | `debug` local (`fbenv`); chart default `warning` |
| `TRADING_ENABLED` | `false` |
| Grafana `BROKER_URL` | `foreverbull:50055` |
| Zipline `DATABASE_URL` | `postgres://foreverbull:foreverbull@postgres:5432/foreverbull` |

Zipline is reached by in-cluster Service `zipline:50055`. Worker pods get
`BROKER_HOSTNAME`, `BROKER_PORT` (gRPC `50055`), `STORAGE_*`, `DATABASE_URL`,
`LOGLEVEL` (`internal/container` `Start`). The broker has no HTTP listener.

Persistence is cluster PVCs (`persistence.*`). Historical Docker host binds
under `.foreverbull/{postgres,minio,nats}` are not used.

## Cluster config

Users pick `cluster: local` (default) or `remote`. `fbenv use` writes the
project file.

| File | Role |
| --- | --- |
| `deploy/cluster-config.example.yaml` | Canonical keys and defaults |
| `.foreverbull/config.yaml` | Project config (cwd; do not commit) |
| `~/.foreverbull/config.yaml` | Optional user override |
| `FOREVERBULL_CONFIG` | Explicit path; wins over the files above |
| `FOREVERBULL_BROKER_ADDRESS` | Runtime broker `host:port`; highest priority |

Merge order (later wins): built-in defaults → user → project → `FOREVERBULL_CONFIG`.

Required keys (see the example file):

| Key | Default | Meaning |
| --- | --- | --- |
| `cluster` | `local` | `local` or `remote` |
| `namespace` | `foreverbull` | Helm namespace for local; remote fallback |
| `kind.name` | `foreverbull` | Kind cluster name |
| `kind.config` | `deploy/kind/cluster.yaml` | Kind config path |
| `remote.kubeconfig` | empty | Empty → `KUBECONFIG` or `~/.kube/config` |
| `remote.context` | empty | Empty → current-context |
| `remote.namespace` | empty | Empty → `namespace` |
| `remote.broker_address` | empty | Required when `cluster=remote` (`host:port`) |

## Kind

Local default is a Kind cluster named `foreverbull`. The node image is **not**
pinned; Kind uses its default Kubernetes version. `extraPortMappings` in
`deploy/kind/cluster.yaml` are 1:1 with the host ports above and are fixed at
create time — delete and recreate the cluster to change them.

Create, image load, and Helm install are idempotent (safe to re-run). Taskfile
wrappers: `task kind:create`, `task kind:load`, `task helm:install`.

## Commands

Chart path: `deploy/helm/foreverbull`. Release name: `foreverbull`. Namespace
comes from the Helm release (`--namespace`, default `foreverbull`).

### Local (default)

CLI (from `client/` after `uv sync`):

```bash
VERSION=local uv run fbenv create   # workloads stay stopped
uv run fbenv start
```

Equivalent by hand:

```bash
# 1. Create the Kind cluster (or: task kind:create)
kind create cluster --name foreverbull --config deploy/kind/cluster.yaml

# 2. Load images tagged VERSION (default local). Or: task kind:load
#    Build first if the images are not on the Docker host:
#      task build:docker
#      task client:docker:zipline
#      task grafana:build:docker
VERSION=local ./deploy/kind/load-images.sh

# 3. Install the chart (or: task helm:install)
helm upgrade --install foreverbull deploy/helm/foreverbull \
  --namespace foreverbull \
  --create-namespace \
  --kube-context kind-foreverbull \
  --set expose.hostPort=true \
  --set global.imageTag=local \
  --set broker.logLevel=debug
```

`load-images.sh` loads `lhjnilsson/foreverbull`, `lhjnilsson/zipline`, and
`lhjnilsson/fb-grafana` at `$VERSION` (or `local`) into the Kind node. Postgres,
NATS, and Minio are pulled by the node on first pod start (or use images already
cached on the Docker host).

Host checks after install: `localhost:5432`, `4222`, `9000`, `50055`, `3000`,
and `27000`–`27015`. Python `fbull` keeps using `localhost:50055` unless
`FOREVERBULL_BROKER_ADDRESS` is set.

Teardown:

```bash
uv run fbenv stop --remove --purge
# or:
helm uninstall foreverbull --namespace foreverbull
kind delete cluster --name foreverbull
```

### Remote

CLI:

```bash
uv run fbenv use remote \
  --kubeconfig /path/to/kubeconfig \
  --context my-context \
  --namespace my-namespace \
  --broker-address host:50055
uv run fbenv create
uv run fbenv start
```

Or install the chart against a real cluster:

```bash
helm upgrade --install foreverbull deploy/helm/foreverbull \
  --kubeconfig /path/to/kubeconfig \
  --kube-context my-context \
  --namespace my-namespace \
  --create-namespace \
  --set expose.hostPort=false
```

Set `cluster: remote` and `remote.broker_address` in config (or
`FOREVERBULL_BROKER_ADDRESS`). Do not pass Alpaca keys as chart values.

Override defaults with `--set` or `-f my-values.yaml`. Check the chart without
a cluster:

```bash
helm lint deploy/helm/foreverbull
helm template foreverbull deploy/helm/foreverbull --namespace foreverbull
bash deploy/helm/check-chart.sh
```

`check-chart.sh` lints, templates with and without `expose.hostPort`, and
asserts the render has ServiceAccount/Role/RoleBinding and does not mention
`/var/run/docker.sock`.

## Helm value keys

One chart, these keys. Match `values.yaml`.

```yaml
global:
  imageTag: latest          # VERSION; local CLI uses local

images:
  postgres:
    repository: postgres
    tag: 13.3-alpine
  nats:
    repository: nats
    tag: 2.10-alpine
  minio:
    repository: pgsty/silo
    tag: RELEASE.2026-09-16T00-00-00Z
  broker:
    repository: lhjnilsson/foreverbull
    tag: ""                 # empty → global.imageTag
  zipline:
    repository: lhjnilsson/zipline
    tag: ""
  grafana:
    repository: lhjnilsson/fb-grafana
    tag: ""

expose:
  hostPort: false           # local CLI sets true; remote leaves false

persistence:
  enabled: true
  postgres:
    size: 8Gi
    path: /pgdata
  nats:
    size: 1Gi
    path: /var/lib/nats/data
  minio:
    size: 8Gi
    path: /data

postgres:
  user: foreverbull
  password: foreverbull
  database: foreverbull
  port: 5432

nats:
  port: 4222
  monitorPort: 8222
  jetstream: true           # command: -js -sd /var/lib/nats/data -m 8222

minio:
  port: 9000
  consolePort: 9001
  accessKey: minioadmin
  secretKey: minioadmin
  command: 'server --console-address ":9001" /data'

broker:
  grpcPort: 50055
  backtestPortStart: 27000
  backtestPortEnd: 27015
  logLevel: warning
  serverAddress: foreverbull
  ziplineServiceName: zipline   # ZIPLINE_CONTAINER_NAME
  trading:
    enabled: false          # TRADING_ENABLED; never default true
    provider: alpaca
    alpaca:
      baseUrl: https://paper-api.alpaca.markets
      apiKey: ""            # no real keys in the chart
      apiSecret: ""

zipline:
  grpcPort: 50055

grafana:
  port: 3000
  brokerUrl: foreverbull:50055
  url: http://localhost:3000  # GRAFANA_URL on the broker; Kind host port

serviceAccount:
  create: true
  name: foreverbull

rbac:
  create: true
```

`replicaCount` is `1`. `global.imagePullPolicy` is `IfNotPresent`. `resources.*`
are modest CPU/memory requests for a local Kind node (limits omitted).

Workload object names (Deployment + Service): `postgres`, `nats`, `minio`,
`zipline`, `foreverbull`, `grafana`.

NATS, Minio, and Postgres keep the commands, credentials, and volume paths
above. Grafana gets `BROKER_URL` from `grafana.brokerUrl`. The broker gets `GRAFANA_URL`
from `grafana.url` for `GetExperiment` deep links. CLI on the laptop still
defaults to `http://localhost:3000`; remote installs should set `grafana.url`
to the public Grafana origin.

## Broker RBAC

The broker ServiceAccount (`serviceAccount.name`) needs a Role in the install
namespace so `internal/container` can Get/Start/StopAll extra pods. Scope is
**that namespace only**.

```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "pods/status"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "watch", "create", "delete"]
```

Worker pods: labels `platform=foreverbull`, `type=service`,
`app.kubernetes.io/name=<name>`. No docker.sock volume on the broker pod.
