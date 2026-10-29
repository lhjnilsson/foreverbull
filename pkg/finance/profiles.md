# Trading profiles and backtest price sources

A deployment names the trading profile it places orders on. A backtest names the price source that filled its bars. The process does not choose one broker or one price feed at startup.

This file is the contract. `pkg/finance/trading.md` still describes the order shape and the four adapters. Where this file and that one disagree about configuration, this file wins. The order shape, the ISIN wrapper, the sentinel errors, and the refusal of `mode=live` do not change.

## What is stored where

| Fact | Where it lives |
| --- | --- |
| Profile name on a running strategy | `Deployment.backend` |
| Price source of a backtest | `Backtest.provider` |
| Adapter, currency, and credentials for a profile | The profiles file, mounted from a Kubernetes Secret |
| Kill switch | `TRADING_ENABLED`, default `false` |
| Paper and demo base URLs | Constants in the adapters |
| ISIN listings | Embedded sample, unless a profile sets a listings path |

The deployment row and the backtest row do not store keys, tokens, or passwords. `.foreverbull/config.yaml` does not store them either. Committed Helm values do not store them.

## Profiles file

Path comes from `TRADING_PROFILES_FILE`. In the cluster that path is `/etc/foreverbull/profiles.yaml`, mounted from Secret `foreverbull-trading`, key `profiles.yaml`. The Secret is optional. When the file is absent and `TRADING_ENABLED` is false, the registry is empty and the process starts. When `TRADING_ENABLED` is true, a missing or invalid file is a startup error.

Local Kind: if `.foreverbull/trading-profiles.yaml` exists, `fbenv` applies it as that Secret before `helm upgrade`. `.foreverbull` is gitignored. The committed example is `deploy/trading-profiles.example.yaml` and its credential fields are empty. `fbenv` must not print the file.

Shape:

```yaml
profiles:
  - name: alpaca-paper
    provider: alpaca
    currency: USD
    apiKey: ""
    apiSecret: ""
  - name: saxo-sim
    provider: saxo
    currency: USD
    accessToken: ""
    accountKey: ""
  - name: ig-demo
    provider: ig
    currency: USD
    apiKey: ""
    identifier: ""
    password: ""
  - name: ibkr-paper
    provider: ibkr
    currency: USD
    accountId: ""
    accessToken: ""
```

Rules:

- `name` is unique. `provider` is `alpaca`, `saxo`, `ibkr`, or `ig`.
- No `baseUrl` field. A file that contains one is invalid. URLs are the paper and demo constants from `trading.md`.
- `ibkr` is paper mode. The file cannot set a live account mode.
- Currency defaults to `USD` when omitted. `GetPortfolio` still returns `ErrCurrency` when the account currency differs.
- Two deployments that name the same profile share that account's positions.

`Deploy` with `dry_run=false` requires `TRADING_ENABLED` and a `backend` equal to a profile name in the registry. An unknown name is `FailedPrecondition`. `dry_run=true` may omit `backend`; the stored value is empty, not `"default"`. `mode=LIVE` stays refused. `TRADING_ALLOW_LIVE_URL` is removed. A live host is not representable in the file.

The runner loads the client for `deployment.backend` on each session. It does not keep a single process-wide trading client. `pkg/strategy` still does not import a broker SDK.

Remove these environment variables and the Helm values that set them: `TRADING_PROVIDER`, `TRADING_ALLOW_LIVE_URL`, `TRADING_CURRENCY`, `ALPACA_MARKETS_BASE_URL`, `ALPACA_MARKETS_API_KEY`, `ALPACA_MARKETS_API_SECRET`, `SAXO_BASE_URL`, `SAXO_ACCESS_TOKEN`, `SAXO_ACCOUNT_KEY`, `IG_BASE_URL`, `IG_API_KEY`, `IG_IDENTIFIER`, `IG_PASSWORD`, `IBKR_BASE_URL`, `IBKR_ACCOUNT_ID`, `IBKR_ACCOUNT_MODE`, `IBKR_ACCESS_TOKEN`. Credentials exist only in the profiles file.

`broker.yaml` must not render those keys as plain `value:` entries. It mounts the Secret and sets `TRADING_PROFILES_FILE` and `TRADING_ENABLED`. `deploy/helm/check-chart.sh` fails if the rendered broker manifest contains an Alpaca, Saxo, IG, or IBKR credential value, or the host `api.alpaca.markets`.

## Price source on a backtest

`Backtest.provider` is `yahoo`, `stooq`, or `alpaca`. Create stores `yahoo` when the caller omits it. The column is `NOT NULL`. Existing rows that predate the column migrate to `yahoo`.

`MARKET_DATA_PROVIDER` is removed. Asset search, `GetAsset`, and `GetIndex` stay on Yahoo. Stooq remains daily `GetOHLC` only, with Yahoo for metadata, chosen because this backtest says `stooq`. Alpaca `GetOHLC` uses the key of a profile:

- The create request may set `profile` when `provider` is `alpaca`.
- If `profile` is empty and exactly one Alpaca profile is configured, use that one.
- Zero Alpaca profiles, or more than one with `profile` empty, fails ingest before any download.
- `profile` on a `yahoo` or `stooq` backtest is ignored.

Stored bars are adjusted per provider as follows. A split never shows as a one-day move:

| Provider | Splits | Dividends | Return basis |
| --- | --- | --- | --- |
| `yahoo` | adjusted (chart `close`) | not adjusted | price return |
| `alpaca` | adjusted (`adjustment=split`, sent explicitly; the SDK default is `raw`) | not adjusted | price return |
| `stooq` | adjusted | adjusted | close to total return |

Bars already stored are not replaced on a later download (`ON CONFLICT DO NOTHING`). Alpaca bars downloaded before this change stay raw. Delete them from the broker's Postgres, then run `fbull backtest delete <name>` and `fbull backtest create` for each Alpaca backtest to fetch them again:

```bash
kubectl exec -n <namespace> deploy/postgres -- \
  psql -U foreverbull -d foreverbull \
  -c "DELETE FROM ohlc WHERE provider = 'alpaca'"
```

No `fbull` or `fbenv` command clears stored bars yet; a fix for replacing stored bars is planned.

`ohlc` is unique on `(provider, symbol, time)`. Existing tables gain `provider text NOT NULL DEFAULT 'yahoo'` and replace `UNIQUE (symbol, time)` with `UNIQUE (provider, symbol, time)`. The migration is idempotent and does not delete bars. `CREATE TABLE IF NOT EXISTS` alone is not a migration.

Every read that decides which bars a backtest or a worker sees filters on that provider:

- `OHLC.Store`, `Exists`, and the execution bundle query.
- Python `Asset.stock_data` and the close lookup used by `order_target_percent`.
- The Zipline bundle query in `client/zipline`.
- The calendar missing-bar check when it is deciding ingest completeness for a backtest.

The worker pod gets `OHLC_PROVIDER` from the backtest when the pod is created. Python and Zipline read that variable and default it to `yahoo` (`DEFAULT_PROVIDER` in `foreverbull.models.finance`) only when it is unset or blank, which covers tests that insert bars and leave the column at its default. Two providers may store two closes for the same symbol and day. A run reads only its own.

Changing `provider` on an existing backtest does not relabel old rows. The next ingest writes the new provider's rows. The run reads the provider stored on the backtest.

## CLI

`fbull backtest create` reads optional `provider` and `profile` from the YAML file. A flag `--provider` overrides the file. An omitted provider is stored as `yahoo`. Create prints `Using provider: <name> (<source>)`, with source `default`, `from --provider`, or `from <YAML file name>`. The printed backtest shows the provider. An unknown name fails with the valid list.

`fbull strategy deploy --no-dry-run` requires `--backend <profile>`. Dry-run may omit it. The help text still says live mode is refused.

## Out of scope

- Enabling live deploy or live base URLs.
- Splitting one profile's positions across deployments.
- A secret manager other than a Kubernetes Secret. The gitignored file is only the install source for that Secret.
- FX conversion, stops, or a second Python order API.
- Rewriting ETF membership or ISIN identity.

## Definition of done

- `TRADING_PROVIDER` and `MARKET_DATA_PROVIDER` are not read.
- A deployment with `backend=alpaca-paper` uses that profile's client. A second deployment can name a different profile.
- Dry-run stores an empty backend. Paper `PlaceOrder` without a known profile is `FailedPrecondition`. `mode=LIVE` is still refused.
- Rendered Helm for the broker has no trading credential `value:` fields. The example profiles file has empty secrets.
- `ohlc` uniqueness includes `provider`. A test stores two closes for one symbol and day under `yahoo` and `stooq`, and a read with `OHLC_PROVIDER=yahoo` returns only the Yahoo close.
- `fbull strategy` tests still refuse live mode.
