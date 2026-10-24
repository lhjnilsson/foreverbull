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
| Foreverbull broker | `foreverbull` | `lhjnilsson/foreverbull:{tag}` | `50055`, `8080`, `27000`–`27015` |
| Grafana | `grafana` | `lhjnilsson/fb-grafana:{tag}` | `3000` |

`{tag}` is `images.*.tag` or `global.imageTag`. The three app images are `global.imageRepository` plus their short `images.*.repository` name (`foreverbull`, `zipline`, `fb-grafana`); a repository name containing `/` is used as-is. `fbenv` sets `global.imageRepository` from its own `IMAGE_REPOSITORY` and `global.imageTag` to the installed `foreverbull` client version, or to `local` with `VERSION=local`; it never uses `latest`.

### Host ports (local Kind)

Published on the host when `expose.hostPort=true` (local CLI):

| Port | Service |
| --- | --- |
| `5432` | Postgres |
| `4222` | NATS |
| `9000` | Minio |
| `50055` | Foreverbull gRPC (Python stubs: `localhost:50055`) |
| `8080` | Foreverbull HTTP (page and `/api`: `localhost:8080`) |
| `27000`–`27015` | Backtest NNG (`BACKTEST_PORT_RANGE_START` / `END`) |
| `3000` | Grafana |

`deploy/kind/cluster.yaml` maps each of those 1:1 onto the Kind control-plane
node (`hostPort` == `containerPort`). The chart sets matching container
`hostPort` when `expose.hostPort=true`. NodePort is not used: these ports are
outside `30000`–`32767`.

Every one of those ports must be free on the host when the Kind cluster is
created. A taken port makes `kind create cluster` fail. See
[Troubleshooting](../README.md#troubleshooting) in the root README for how to
find the process and recreate the cluster, and
[Host resources](../README.md#host-resources) for the CPU, memory, and disk the
six workloads need.

Remote installs leave `expose.hostPort=false` (ClusterIP only). Point the CLI
and Python stubs at `remote.broker_address` or `FOREVERBULL_BROKER_ADDRESS`.

On local Kind, `5432`, `4222`, `9000`, and `3000` ignore role. Sharing a
backtest does not close them. See [Authentication](#authentication).

### In-cluster env (broker)

Matches `internal/environment/env.go` plus chart overrides:

| Variable | Value |
| --- | --- |
| `POSTGRES_URL` | `postgres://foreverbull:$(POSTGRES_PASSWORD)@postgres:5432/foreverbull` |
| `NATS_URL` | `nats://nats:4222`. The password is not in this URL |
| `NATS_USER` / `NATS_PASSWORD` | User `broker`. The password is Secret `foreverbull-nats` key `password` |
| `WORKER_DB_PASSWORD` | Secret `foreverbull-worker` key `password`. Not the database owner password |
| `WORKER_DATABASE_URL` | `postgres://foreverbull_worker:$(WORKER_DB_PASSWORD)@postgres:5432/foreverbull`. The broker sends this to a service pod |
| `MINIO_URL` | `minio:9000` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET` | From Secret `foreverbull-credentials`. Local Kind: `minioadmin` / `minioadmin` |
| `SERVER_ADDRESS` | `foreverbull` |
| `ZIPLINE_CONTAINER_NAME` | `zipline` |
| `POD_NAMESPACE` | pod namespace (fieldRef) |
| `GRPC_PORT` | `50055` |
| `HTTP_PORT` | `8080` |
| `BACKTEST_PORT_RANGE_START` / `END` | `27000` / `27015` |
| `LOG_LEVEL` | `debug` local (`fbenv`); chart default `warning` |
| `TRADING_ENABLED` | `false` |
| `AUTH_ENABLED` | unset. Chart `broker.auth.enabled` is `false`. `fbenv` does not set it |
| `AUTH_ADMIN_USERNAME` | empty unless `broker.auth.adminUsername` is set |
| `AUTH_ADMIN_PASSWORD_FILE` | `/etc/foreverbull-admin/password`. The value is the path. Secret `foreverbull-admin` key `password` is optional and is not an environment variable |
| `AUTH_COOKIE_SECURE` | unset (`false`). Set `true` when TLS terminates in front of the broker. Plain HTTP, including local Kind, leaves the cookie unsecured |
| `TRADING_PROVIDER` | `alpaca` (`alpaca`, `saxo`, `ibkr`, or `ig`) |
| `TRADING_CURRENCY` | `USD` |
| `TRADING_ALLOW_LIVE_URL` | `false`. Does not enable deploy mode LIVE |
| `ALPACA_MARKETS_BASE_URL` | `https://paper-api.alpaca.markets` (paper) |
| `SAXO_BASE_URL` | `https://gateway.saxobank.com/sim/openapi` (simulation) |
| `IG_BASE_URL` | `https://demo-api.ig.com/gateway/deal` (demo) |
| `IBKR_BASE_URL` | `https://localhost:5000/v1/api` |
| `IBKR_ACCOUNT_MODE` | `paper` |
| Grafana `BROKER_URL` | `foreverbull:50055` |
| Zipline `DATABASE_URL` | `postgres://foreverbull:$(POSTGRES_PASSWORD)@postgres:5432/foreverbull` |

Deploy mode LIVE is refused. Trading credentials are not chart values: optional Secret `foreverbull-trading` key `profiles.yaml` is mounted at `/etc/foreverbull/profiles.yaml` (`TRADING_PROFILES_FILE`). `fbenv create` and `fbenv update` apply `.foreverbull/trading-profiles.yaml` as that Secret when the file exists, and do not print it. The committed example is `deploy/trading-profiles.example.yaml`.

Zipline is reached by in-cluster Service `zipline:50055`. Service pods get
`BROKER_HOSTNAME`, `BROKER_PORT` (gRPC `50055`), `STORAGE_ENDPOINT`, and
`LOGLEVEL` (`internal/container` `Start`). They do not get `DATABASE_URL` or
the MinIO keys. The broker has no HTTP listener.

Persistence is cluster PVCs (`persistence.*`). Historical Docker host binds
under `.foreverbull/{postgres,minio,nats}` are not used.

### Credentials

Postgres and MinIO credentials live in Secret `foreverbull-credentials` (keys
`postgres-password`, `minio-accessKey`, `minio-secretKey`). Pods read them with
`secretKeyRef`; `POSTGRES_PASSWORD` is expanded into `POSTGRES_URL` and
`DATABASE_URL`. Leave `postgres.password`, `minio.accessKey`, and
`minio.secretKey` empty and the chart picks them:

- `credentials.allowDefaults=true` (local Kind; `fbenv` passes it
  explicitly): `foreverbull` and `minioadmin` / `minioadmin`, so host
  tools on `localhost` keep working. `expose.hostPort` is networking
  only: it publishes ports, is for local Kind only, and never selects
  these defaults on its own.
- Otherwise: a random password and MinIO secret key (access key
  `foreverbull`), generated on the first install and read back from the Secret
  on every upgrade.

A value you set wins. `postgres.password` and `minio.secretKey` must be
URL-safe (letters, digits and `-._~` are safe): the Postgres password is
expanded raw into `POSTGRES_URL`, and the chart now fails the install or
upgrade on any other character instead of rendering a broken DSN.

Generated credentials are not printed again, so read them back from the
Secret. Replace `<namespace>` with the Helm release namespace
(`foreverbull` on local Kind, your `--namespace` on a remote install):

```bash
kubectl get secret foreverbull-credentials -n <namespace> -o jsonpath='{.data.postgres-password}' | base64 -d; echo
kubectl get secret foreverbull-credentials -n <namespace> -o jsonpath='{.data.minio-accessKey}' | base64 -d; echo
kubectl get secret foreverbull-credentials -n <namespace> -o jsonpath='{.data.minio-secretKey}' | base64 -d; echo
```

Generated values stay stable across `helm upgrade`: the chart looks up
the existing Secret `foreverbull-credentials` in the release namespace
and reuses its values before generating new ones. That lookup only
works against a live cluster, so an offline `helm template` render
generates fresh values every time; do not apply such a render over a
live release, or the Secret would rotate while Postgres keeps the
password it was initialised with. Likewise, deleting Secret
`foreverbull-credentials` while the PVCs survive breaks auth: the next
upgrade generates fresh values that the already-initialised Postgres
does not know.

🔒️ With or without `expose.hostPort`, a set `postgres.password=foreverbull`
or `minio.secretKey=minioadmin` stops the install unless
`credentials.allowDefaults=true` is set explicitly. A remote release
installed before this change needs the migration below, not a plain
upgrade.

## Authentication

`AUTH_ENABLED` defaults to false. An empty value is false. Helm and `fbenv`
do not set it. Chart `broker.auth.enabled` stays `false`. A fresh install is
the open broker: HTTP `8080`, gRPC `50055`, and session ports `27000`–`27015`
do not require a credential.

The first start with `AUTH_ENABLED=true` refuses to serve until
`AUTH_ADMIN_USERNAME` is set and, when that account has no password row,
`AUTH_ADMIN_PASSWORD_FILE` can be read. That boot assigns existing services,
backtests, and strategies to that admin, moves `service` and `backtest` onto
uuid primary keys, copies each MinIO ingestion object from the name key to
the id key, and then requires a credential on HTTP, broker gRPC, and those
session ports. A failed copy leaves the name primary key and the name object.
Rows are not deleted. A failed start serves nothing.

Snapshot Postgres and the MinIO ingestions prefix before that boot. Restoring
the snapshot is the rollback. An image rollback without it is not one.

The admin password is the file from Secret `foreverbull-admin`, key
`password`, mounted at `/etc/foreverbull-admin/password`, beside the profiles
mount rather than inside it. It is not an
environment variable, not a CLI flag, and not a key in
`.foreverbull/config.yaml`. The broker hashes the file and does not log the
file or the hash. `broker.auth.adminUsername` is the login, not the password.
`fbenv` does not create the Secret. Create it from a file outside the repo,
mode `0600`, before the enable boot. Replace `<namespace>` with the Helm
release namespace:

```bash
kubectl create secret generic foreverbull-admin \
  --namespace <namespace> \
  --from-file=password=./admin-password
```

Then set the chart values on an upgrade you run yourself. Do not pass the
password with `--set`:

```bash
helm upgrade foreverbull deploy/helm/foreverbull \
  --namespace <namespace> \
  --reuse-values \
  --set broker.auth.enabled=true \
  --set broker.auth.adminUsername=<login>
```

A later start with `AUTH_ENABLED` false, after `owner_id` is `NOT NULL` on
`service`, `backtest`, or `strategy`, exits. Turning the flag off is not a
rollback.

On local Kind, host ports `5432`, `4222`, `9000`, and `3000` ignore role.
Sharing a backtest does not close them. A remote install with
`expose.hostPort=false` does not publish those ports, or the session range,
on the host. A remote broker that still publishes `8080` and `50055` stays
open until `AUTH_ENABLED=true` and the admin are set. An unset flag is not
"authentication was configured".

A service image does not receive the database owner URL or the MinIO root
keys. The worker reads `asset` and `ohlc` as role `foreverbull_worker`. That
role is not granted `account`, `api_token`, `browser_session`, `oauth_state`,
or `domain_grant`. Its password is Secret `foreverbull-worker`. NATS
authenticates the broker user from Secret `foreverbull-nats`. Service pods
have no NATS user. Their NetworkPolicy allows DNS, the broker (gRPC and the
session ports the worker dials), and Postgres. It does not allow MinIO or
NATS. The algorithm still reads bars with SQL, so Postgres stays reachable
as that role. The Zipline deployment is the platform image and still uses
the database owner URL. A command that names a backtest id is refused when
its `owner_id` is not the owner stored on that row.

Session secrets are kept in the broker process. Run one broker replica. A
second replica does not have the map and refuses a session it did not mint.

CLI tokens last until they are revoked. `expires_at` is checked and mint does
not set it. `account rotate` replaces the password hash and deletes that
account's browser sessions. It has no current session to keep. API tokens
stay until they are revoked, so a forgotten password does not remove the
Grafana token. Behind TLS, set `AUTH_COOKIE_SECURE=true`. The chart does not set it.
Broker gRPC is plaintext, so a token or session secret on a `broker_address`
you do not trust can be read on the path. Do not publish `50055` or `8080`
beyond that network.

## Upgrading an existing remote install

Read this before the first upgrade of a remote release that was
installed before credentials moved into Secret `foreverbull-credentials`.
That release has Postgres data initialised with the old password, and
the Postgres image only applies `POSTGRES_PASSWORD` on first init, so
a generated random password in a new Secret would silently lock every
client out. The chart fails such an upgrade until the old credentials
are migrated:

1. On the first upgrade, store the old credentials in the Secret: set
   `postgres.password`, `minio.accessKey`, and `minio.secretKey` to the
   values the existing install actually uses (before this change, the
   chart defaults `foreverbull` and `minioadmin` / `minioadmin`), and
   add `credentials.allowDefaults=true` when those are the public
   defaults. Upgrade with Helm directly, for example:

   ```bash
   helm upgrade --install foreverbull deploy/helm/foreverbull \
     --namespace <namespace> \
     --create-namespace \
     --set expose.hostPort=false \
     --set postgres.password='<old-postgres-password>' \
     --set minio.accessKey='<old-minio-access-key>' \
     --set minio.secretKey='<old-minio-secret-key>' \
     --set credentials.allowDefaults=true
   ```

2. Confirm the workloads are healthy and that Secret
   `foreverbull-credentials` in that namespace now holds those values
   (read them back with the `kubectl get secret` commands above).
3. Rotate the password inside Postgres afterwards, while the old
   password still works, and choose a new URL-safe password:

   ```bash
   kubectl exec -n <namespace> deploy/postgres -- \
     psql -U foreverbull -d foreverbull \
     -c "ALTER USER foreverbull WITH PASSWORD '<new-url-safe-password>'"
   ```

4. Upgrade again with `postgres.password` set to that new password
   (and a new URL-safe `minio.secretKey` if you rotate MinIO too), this
   time without `credentials.allowDefaults=true`, so the Secret and
   Postgres agree and no public default remains.

Always install and upgrade with Helm directly. Never apply
`helm template` output to a live release for credentials: offline
renders cannot look up the existing Secret and generate a new random
password on every render.

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

Without `VERSION`, a local `fbenv create` pulls the three app images tagged with the installed `foreverbull` client version and stops before installing anything when one is missing. That promise is local-Kind only: on a remote cluster this host does not pull the images, so Helm can create or update the release first and the remote nodes can then fail to pull, leaving pods in `ImagePullBackOff`. `VERSION=local` uses the `:local` images built from this checkout; `fbenv` refuses any other `VERSION`.

From the repository root, `task kind:redeploy` does the local rebuild in one step. It builds the `:local` images, deletes the Kind cluster `foreverbull` when it exists, installs the chart, and starts the workloads. The purge also removes `client/.foreverbull` data other than `config.yaml`.

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
  --set credentials.allowDefaults=true \
  --set global.imageRepository=lhjnilsson \
  --set global.imageTag=local \
  --set broker.logLevel=debug
```

`load-images.sh` loads `lhjnilsson/foreverbull`, `lhjnilsson/zipline`, and
`lhjnilsson/fb-grafana` at `$VERSION` (or `local`) into the Kind node. Postgres,
NATS, and Minio are pulled by the node on first pod start (or use images already
cached on the Docker host).

Host checks after install: `localhost:5432`, `4222`, `9000`, `50055`, `8080`, `3000`,
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
  imageRepository: lhjnilsson  # fbenv passes its IMAGE_REPOSITORY here
  imageTag: latest          # fbenv sets the client version, or local

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
    repository: foreverbull  # short name → global.imageRepository/foreverbull
    tag: ""                 # empty → global.imageTag
  zipline:
    repository: zipline
    tag: ""
  grafana:
    repository: fb-grafana
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

credentials:
  allowDefaults: false      # fbenv sets true for local Kind; else throwaway only

postgres:
  user: foreverbull
  password: ""              # empty → see Credentials
  database: foreverbull
  port: 5432

nats:
  port: 4222
  monitorPort: 8222
  jetstream: true           # command: -js -sd /var/lib/nats/data -m 8222

minio:
  port: 9000
  consolePort: 9001
  accessKey: ""             # empty → see Credentials
  secretKey: ""
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
    provider: alpaca        # TRADING_PROVIDER: alpaca, saxo, ibkr, or ig
    currency: USD           # TRADING_CURRENCY
    allowLiveURL: false     # TRADING_ALLOW_LIVE_URL; does not unlock mode LIVE
    listingsFile: ""        # TRADING_LISTINGS_FILE; empty uses the embedded sample
    alpaca:
      baseUrl: https://paper-api.alpaca.markets
      apiKey: ""            # ALPACA_MARKETS_API_KEY; no keys in the chart
      apiSecret: ""         # ALPACA_MARKETS_API_SECRET; no secrets in the chart
    saxo:
      baseUrl: https://gateway.saxobank.com/sim/openapi
      accessToken: ""       # SAXO_ACCESS_TOKEN; no tokens in the chart
      accountKey: ""        # SAXO_ACCOUNT_KEY; empty uses the client default account
    ig:
      baseUrl: https://demo-api.ig.com/gateway/deal
      apiKey: ""            # IG_API_KEY; no keys in the chart
      identifier: ""        # IG_IDENTIFIER; empty
      password: ""          # IG_PASSWORD; no passwords in the chart
    ibkr:
      baseUrl: https://localhost:5000/v1/api
      accountId: ""         # IBKR_ACCOUNT_ID; required when provider is ibkr
      accountMode: paper    # IBKR_ACCOUNT_MODE; account mode live is refused unless allowLiveURL. That flag does not unlock deploy mode LIVE.
      accessToken: ""       # IBKR_ACCESS_TOKEN; no tokens in the chart
  auth:
    enabled: false          # AUTH_ENABLED; fbenv leaves this false
    adminUsername: ""       # AUTH_ADMIN_USERNAME; not the password

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
