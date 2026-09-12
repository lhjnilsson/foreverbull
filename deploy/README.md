# Cluster contract (Kind + Helm)

This directory is the install contract. Later steps add one Helm chart and
switch the CLI from Docker `fbenv` to Kind/Helm. This step does **not** add
the chart or change application code.

- Local default: Kind cluster `foreverbull`, namespace `foreverbull`, one chart.
- Remote: `helm upgrade --install` against a user kubeconfig/context/namespace.
- Broker does **not** mount `/var/run/docker.sock`. Extra strategy-worker pods
  use in-cluster Kubernetes (ServiceAccount RBAC).
- Go/Python Testcontainers stay on Docker. Do **not** convert unit/component
  tests to Kind.

## Six workloads

Same processes as `fbenv` (`client/cli/src/foreverbull_cli/env/environment.py`).
In-cluster DNS names stay the Docker hostnames.

| Workload | K8s Service DNS | Image (default) | Host port |
| --- | --- | --- | --- |
| Postgres | `postgres` | `postgres:13.3-alpine` | `5432` |
| NATS JetStream | `nats` | `nats:2.10-alpine` | `4222` |
| Minio | `minio` | `minio/minio:latest` | `9000` |
| Zipline | `zipline` | `lhjnilsson/zipline:{tag}` | none (in-cluster gRPC `:50055`) |
| Foreverbull broker | `foreverbull` | `lhjnilsson/foreverbull:{tag}` | `50055`, `27000`–`27015` |
| Grafana | `grafana` | `lhjnilsson/fb-grafana:{tag}` | `3000` |

`{tag}` is `images.*.tag` or `global.imageTag` (`VERSION` in fbenv: `latest` or `local`).

### Host ports (local Kind)

Published on the host, same as fbenv:

| Port | Service |
| --- | --- |
| `5432` | Postgres |
| `4222` | NATS |
| `9000` | Minio |
| `50055` | Foreverbull gRPC (Python stubs: `localhost:50055`) |
| `27000`–`27015` | Backtest NNG (`BACKTEST_PORT_RANGE_START` / `END`) |
| `3000` | Grafana |

`deploy/kind/cluster.yaml` maps each of those 1:1 onto the Kind control-plane
node (`hostPort` == `containerPort`). The chart MUST set matching container
`hostPort` when `expose.hostPort=true` (local CLI). NodePort is not used:
these ports are outside `30000`–`32767`.

Remote installs leave `expose.hostPort=false` (ClusterIP only). The CLI talks
to `remote.broker_address` instead of `localhost:50055`.

### In-cluster env (broker)

Matches fbenv container env / `internal/environment/env.go`:

| Variable | Value |
| --- | --- |
| `POSTGRES_URL` | `postgres://foreverbull:foreverbull@postgres:5432/foreverbull` |
| `NATS_URL` | `nats://nats:4222` |
| `MINIO_URL` | `minio:9000` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET` | `minioadmin` / `minioadmin` |
| `SERVER_ADDRESS` | `foreverbull` |
| `GRPC_PORT` | `50055` |
| `HTTP_PORT` | `8080` |
| `BACKTEST_PORT_RANGE_START` / `END` | `27000` / `27015` |
| `LOG_LEVEL` | `debug` local; chart default `warning` |
| `TRADING_ENABLED` | `false` |
| Grafana `BROKER_URL` | `foreverbull:50055` |
| Zipline `DATABASE_URL` | `postgres://foreverbull:foreverbull@postgres:5432/foreverbull` |

Zipline is reached by in-cluster Service `zipline:50055`, not Docker inspect
of `foreverbull_zipline`. Worker pods get the same env `container.Start`
sets today (`BROKER_HOSTNAME`, `BROKER_PORT`, `STORAGE_*`, `DATABASE_URL`,
`LOGLEVEL`).

Persistence is cluster PVCs (`persistence.*`), not host binds to
`.foreverbull/{postgres,minio,nats}`.

## Cluster config

Users pick `cluster: local` (default) or `remote`.

| File | Role |
| --- | --- |
| `deploy/cluster-config.example.yaml` | Canonical keys and defaults |
| `.foreverbull/config.yaml` | Project config (cwd; do not commit) |
| `~/.foreverbull/config.yaml` | Optional user override |
| `FOREVERBULL_CONFIG` | Explicit path; wins over the files above |

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

## Commands

Chart path for later steps: `deploy/helm/foreverbull`. Release name: `foreverbull`.

### Local (default)

```bash
kind create cluster --name foreverbull --config deploy/kind/cluster.yaml

helm upgrade --install foreverbull deploy/helm/foreverbull \
  --namespace foreverbull \
  --create-namespace \
  --set expose.hostPort=true \
  --set global.imageTag=local
```

Host checks after install: `localhost:5432`, `4222`, `9000`, `50055`, `3000`,
and `27000`–`27015`. Python `fbull` keeps using `localhost:50055`.

Teardown:

```bash
helm uninstall foreverbull --namespace foreverbull
kind delete cluster --name foreverbull
```

### Remote

```bash
helm upgrade --install foreverbull deploy/helm/foreverbull \
  --kubeconfig /path/to/kubeconfig \
  --kube-context my-context \
  --namespace my-namespace \
  --create-namespace \
  --set expose.hostPort=false
```

Set `cluster: remote` and `remote.broker_address` in config. Do not pass
Alpaca keys as chart values.

## Helm value keys

One chart, these keys. Do not rename them in later steps.

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
    repository: minio/minio
    tag: latest
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
  httpPort: 8080
  backtestPortStart: 27000
  backtestPortEnd: 27015
  logLevel: warning
  serverAddress: foreverbull
  trading:
    enabled: false          # TRADING_ENABLED; never default true
    provider: alpaca
    alpaca:
      baseUrl: https://paper-api.alpaca.markets
      apiKey: ""            # no real keys in the chart
      apiSecret: ""

grafana:
  port: 3000
  brokerUrl: foreverbull:50055

serviceAccount:
  create: true
  name: foreverbull

rbac:
  create: true
```

Workload object names (Deployment or StatefulSet + Service): `postgres`,
`nats`, `minio`, `zipline`, `foreverbull`, `grafana`.

NATS, Minio, and Postgres keep the fbenv commands, credentials, and volume
paths above. Grafana gets only `BROKER_URL` from `grafana.brokerUrl`.

## Broker RBAC

The broker ServiceAccount (`serviceAccount.name`) needs a Role in the install
namespace to create/get/delete strategy-worker pods (today:
`internal/container` `Start` / `Get` / `Stop` via Docker). Scope is **that
namespace only**.

```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "pods/status"]
    verbs: ["get", "list", "watch", "create", "delete"]
```

Worker pods: labels `platform=foreverbull`, `type=service`. No docker.sock
volume on the broker pod.

## Out of scope here

- Helm chart templates (`deploy/helm/foreverbull`)
- Go Kubernetes runtime replacing `internal/container`
- CLI Kind/Helm commands replacing Docker `fbenv`
- Live trading / Alpaca secrets
- Moving Testcontainers tests onto Kind
