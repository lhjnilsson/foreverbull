# Trading adapters

One interface in front of Alpaca, Saxo, Interactive Brokers, and IG. A strategy emits a share delta for an instrument. The process that calls `supplier.Trading` resolves identity and talks to the broker. The strategy never imports a broker SDK and never branches on the provider name.

This file is the contract. Implementation that contradicts it is wrong.

## Who calls what

- Python `Portfolio.order_target` and `order_target_percent` stay as they are. They still emit `symbol` plus a signed share `amount`. They do not learn ISINs, epics, UICs, or contract ids.
- `pkg/strategy` calls `supplier.Trading` only. It must not import `alpaca`, `saxo`, `ibkr`, `ig`, or any broker HTTP package.
- `pkg/finance/module.go` selects the adapter from `TRADING_PROVIDER` and wraps it so every order has an ISIN before the adapter sees it.
- `TRADING_ENABLED` defaults to `false`. When it is false the module returns a nil `supplier.Trading`, which is what it does today.
- Deploy mode LIVE stays refused (`mode=LIVE`), including when `confirm` is set. Paper `dry_run=false` still requires `confirm` and a non-nil Trading backend.

## Platform order

`finance.Order` gains fields. Do not renumber existing fields.

| Field | Meaning |
| --- | --- |
| `symbol` | Ticker alias the algorithm already uses. Not identity. |
| `amount` | Signed share delta of the underlying. `amount` is a signed share delta: positive buys, negative sells. Zero is rejected locally. |
| `type` | `MARKET` or `LIMIT` only. |
| `time_in_force` | Empty or `DAY`. Any other value is rejected locally. |
| `intent` | Ignored. Adapters must not read it and must not send it. |
| `isin` | Platform identity. Adapters refuse an order whose ISIN is empty. |
| `limit_price` | Required when type is `LIMIT`. Must be greater than zero. Absent for `MARKET`. |
| `status` | `ACCEPTED`, `REJECTED`, `FILLED`, `PARTIALLY_FILLED`. |
| `filled_amount` | Signed shares filled. Zero when the broker has not filled yet. |
| `average_price` | Present when `filled_amount` is non-zero. |
| `reject_reason` | Set when status is `REJECTED` or the call returns an error. |
| `broker_order_id` | Broker id when the broker accepted the order. |

`Position` gains `isin`. `symbol` remains the current ticker alias.

Orders stored on `strategy_deployment_run.orders` stay a JSON blob of this proto. Do not add SQL columns.

## Interface

```go
type Trading interface {
    GetPortfolio(ctx context.Context) (*pb.Portfolio, error)
    PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error)
    GetOrders(ctx context.Context) ([]*pb.Order, error)
}
```

Regenerate `pkg/finance/supplier/mock_Trading.go` with mockery (`task mock:generate`). Do not hand-edit that file. Update every other implementer and caller in the same change so `go test` compiles. Known callers include `pkg/strategy/internal/runner`, `pkg/strategy/internal/api`, and `pkg/finance/internal/api/session.go`.

`session.PlaceOrders` returns the order from `PlaceOrder`, including status and broker id, not the unmodified request.

A canceled context returns `ctx.Err()` and performs no HTTP.

## Identity

ISIN is the adapter identity and the only id an adapter accepts. The wrapper in front of every adapter does this:

1. On `PlaceOrder`, if `isin` is empty, resolve `symbol` through the directory at the current time.
2. Zero matches, or more than one, returns `ErrNotTradable` and does not call the adapter.
3. One match copies that ISIN onto the order and then calls the adapter.
4. The adapter also returns `ErrNotTradable` when `isin` is empty, so skipping the wrapper still fails closed.
5. On `GetPortfolio`, every broker position is mapped back to an ISIN. A position that does not map returns `ErrUnmappedPosition` and fails the whole snapshot. Do not drop it.

The directory reads the existing `instrument` and ticker-alias tables when a database is configured. Tests use an in-memory directory. A symbol that changed (FB to META) resolves to one ISIN.

## Listings

A listing maps one ISIN to broker ids. The sample set is embedded in the trading package. `TRADING_LISTINGS_FILE` replaces it when set. The sample is a test fixture, not a market catalog.

Each row:

| Column | Use |
| --- | --- |
| `isin` | Platform id. |
| `symbol` | Current ticker alias. |
| `currency` | What this row is quoted in. |
| `alpaca_symbol` | Alpaca equity symbol. Empty means Alpaca cannot trade it. |
| `saxo_uic` | Saxo UIC. Zero means Saxo cannot trade it. |
| `saxo_asset_type` | Saxo asset type string for that UIC. |
| `ibkr_conid` | IBKR contract id. Zero means IBKR cannot trade it. |
| `ig_epic` | IG epic. Empty means IG cannot trade it. |
| `ig_share_size` | Underlying shares represented by one IG deal unit. Zero means the epic is not share-mappable. |

Sample rows the tests must include:

- `US0378331005` AAPL, USD, tradable on all four. IG share size `1`.
- `US5949181045` MSFT, USD, tradable on all four. IG share size `1`.
- One ISIN whose `ig_share_size` is `10`, used only to prove a non-divisible delta is rejected.

Broker ids in the sample must be the ids the adapter sends. Tests assert the outbound body contains the id from the listing row, not a constant buried in the adapter.

## Errors

Sentinel errors in `pkg/finance/supplier`, wrapped with `%w` so `errors.Is` works.

| Error | When | HTTP |
| --- | --- | --- |
| `ErrNotTradable` | Unknown ISIN, empty broker id, or empty ISIN at the adapter. | No order POST. |
| `ErrUnsized` | IG share delta is not an integer number of deal units. | No order POST. |
| `ErrRejected` | Amount is zero, limit price is missing or not positive, time in force is not day, or the broker refuses the order. | Broker was called only in the broker-refuse case. |
| `ErrUnmappedPosition` | A live position cannot be mapped to an ISIN. | GetPortfolio fails. |
| `ErrCurrency` | Account currency is not `TRADING_CURRENCY`. | GetPortfolio fails. |
| `ErrUnavailable` | Network failure, HTTP 401, or HTTP 5xx. | No retry. |

`PlaceOrder` returns `(nil, err)` for the sentinel cases. A broker refusal still returns `ErrRejected`. Do not return a rejected order with a nil error.

The strategy runner already stops the session on the first `PlaceOrder` error and does not retry. Keep that. `ErrNotTradable` and `ErrRejected` use that same path.

## Portfolio

`GetPortfolio` fills `cash`, `portfolio_value`, `timestamp`, and positions (`isin`, `symbol`, `amount`, `cost_basis` when the broker sends one).

`TRADING_CURRENCY` defaults to `USD`. The adapter reads the account currency and returns `ErrCurrency` when it differs. It does not convert FX.

`GetOrders` returns the broker's open orders mapped into the same `Order` shape, including `broker_order_id` and status. An empty slice is allowed only when the broker has no open orders. The current Alpaca client returns an empty slice without reading the response. That is a bug. Fix it.

## Adapter rules

All four adapters use `net/http` and `encoding/json`. Inject `*http.Client`. Adapter tests use `httptest`. Do not add a broker SDK. The Alpaca trading client stops importing `alpaca-trade-api-go`. Market-data code may keep that SDK.

Constructors take config, the listing book, and the HTTP client. They do not read the environment. `module.go` is the only place that reads trading env vars.

Signed `amount` becomes a side plus a positive quantity:

- Alpaca: `side` `buy` or `sell`, `qty` absolute, `type` `market` or `limit`, `time_in_force` `day`, `limit_price` only for limit. Headers `APCA-API-KEY-ID` and `APCA-API-SECRET-KEY`. Do not send `position_intent`.
- Saxo: `BuySell` from the sign, `Amount` absolute, `Uic` and `AssetType` from the listing, `OrderType` `Market` or `Limit`. `Authorization: Bearer`. Simulation prefix is the default base URL.
- IBKR Client Portal: `side` `BUY` or `SELL`, `quantity` absolute, `conid` from the listing, `orderType` `MKT` or `LMT`, `tif` `DAY`. If the gateway answers with a confirmation prompt, POST that reply once. A second prompt returns `ErrRejected`. Do not loop.
- IG: deal size is `abs(amount) / ig_share_size`. It must be a positive integer. Otherwise `ErrUnsized` and no POST. Session login is one POST at construction. HTTP 401 later is `ErrUnavailable`, not another login loop.

Confirm request field names against the official broker docs while implementing, and cite the doc URL in a comment on the fixture. The fixture is what the test asserts byte for byte on the fields that carry identity, side, quantity, and limit price. If the docs cannot be opened, stop. Do not invent a body.

Partial fills: `filled` non-zero and smaller than the requested absolute quantity becomes `PARTIALLY_FILLED`. No fill yet is `ACCEPTED` with `filled_amount` 0. A full fill is `FILLED`.

## Live gates

Two gates, and they do different jobs.

1. Strategy mode LIVE stays refused in `Deploy` (`mode=LIVE`). Do not remove that check.
2. The factory refuses known live endpoints unless `TRADING_ALLOW_LIVE_URL=true`. Default is false. Setting that flag still does not make `mode=LIVE` succeed and does not unlock mode LIVE.

| Provider | Default base URL | Refused unless the flag is set |
| --- | --- | --- |
| `alpaca` | `https://paper-api.alpaca.markets` | host `api.alpaca.markets` |
| `saxo` | `https://gateway.saxobank.com/sim/openapi` | path without `/sim/` |
| `ig` | `https://demo-api.ig.com/gateway/deal` | host `api.ig.com` |
| `ibkr` | `https://localhost:5000/v1/api` | `IBKR_ACCOUNT_MODE=live` |

`IBKR_ACCOUNT_MODE` defaults to `paper`. The adapter will not place an order when the mode is unset.

Unknown `TRADING_PROVIDER` is a startup error. Known values: `alpaca`, `saxo`, `ibkr`, `ig`.

Empty secrets stay empty. Helm values must not contain API keys, tokens, or passwords. Document the variable names. Do not copy secrets into `values.yaml`, tests, or fixtures.

## Tests

Adapter tests must pass with no Docker, no Kind, and no network. `httptest` only.

Shared cases, one test function per case, run for every adapter:

1. Market buy of 10 AAPL shares. Outbound body carries the listing id, buy side, and quantity 10. Response maps to `ACCEPTED` or `FILLED` with `broker_order_id` set.
2. Market sell is a negative `amount` and a sell side. Quantity on the wire is positive.
3. Limit order carries `limit_price`. A limit order without a price returns `ErrRejected` and does not POST.
4. Unknown ISIN returns `ErrNotTradable` and does not POST an order.
5. Amount 0 returns `ErrRejected` and does not POST.
6. Broker HTTP 400 returns `ErrRejected`.
7. Broker HTTP 500 returns `ErrUnavailable`.
8. Canceled context does not POST.
9. `GetPortfolio` maps cash, equity, and one position back to the sample ISIN and symbol.
10. A position whose broker id is not in the listing returns `ErrUnmappedPosition`.
11. Account currency other than `TRADING_CURRENCY` returns `ErrCurrency`.
12. `GetOrders` maps one open order, including broker id. It does not return an empty slice when the fixture has an order.

IG also: a share delta that is not a whole number of `ig_share_size` returns `ErrUnsized` and does not POST.

IBKR also: one confirmation prompt is answered once, and the order then succeeds. A second prompt returns `ErrRejected`.

Factory also: each live URL or `IBKR_ACCOUNT_MODE=live` fails construction when `TRADING_ALLOW_LIVE_URL` is unset, and succeeds when it is `true`. Paper URLs succeed without the flag.

Wrapper also: FB and META as two aliases of one ISIN produce one order ISIN. Two ISINs for one symbol return `ErrNotTradable`.

Runner also: `ErrNotTradable` from `PlaceOrder` persists the attempted order, stops the session, and does not send the next order. `mode=LIVE` is still `FailedPrecondition`.

## Out of scope

- Enabling live deploy.
- Stops, brackets, guaranteed stops, margin, or multi-currency conversion.
- Streaming prices.
- A second order API for Python.
- Changing `order_target_percent` or backtest fills.
- Kind or a real broker account in CI.
- Nordnet, Avanza, Tradier, or Schwab.
- Retrying a failed order.

## Definition of done

- `supplier.Trading` methods take `context.Context`.
- All four providers construct through `module.go` and the ISIN wrapper.
- The twelve shared cases exist for each provider, plus the IG, IBKR, factory, wrapper, and runner cases above.
- `go test ./pkg/finance/internal/suppliers/trading/ ./pkg/finance/supplier/ ./pkg/strategy/internal/runner/` passes without Docker.
- `pkg/strategy` has no broker SDK import.
- `Deploy` refuses live mode.
- Default Alpaca URL is the paper host. Default Saxo URL contains `/sim/`. Default IG URL contains `demo-api.ig.com`.
- No secret values were added to Helm values, fixtures, or tests.
