# Cookiecutter algorithm templates

Foreverbull ships two cookiecutter templates under this directory. Generated
strategies must look like something you would actually backtest: they size from
`portfolio_value`, leave a cash buffer, cap how many names they hold, rebalance
every bar, and flatten names that leave the set.

| Template | Path | Indicators |
| --- | --- | --- |
| **simple** | `simple/` | pandas SMA crossover only — **no TA-Lib** |
| **advanced** | `advanced/` | TA-Lib (`macd.py`, `rsi.py`) |

Do not add TA-Lib to the simple template. Do not add a third template until
both of these size and rebalance realistically.

Generate from `client/`:

```bash
uv run cookiecutter templates/simple
uv run cookiecutter templates/advanced
```

Keep cookiecutter `{{ }}` path names intact when editing the templates.

## Sizing contract

Every generated `handle_data` / `place_orders` (or equivalent) must:

1. Read current `portfolio_value` (via `Portfolio.order_target_percent`, which
   already multiplies `percent` by `portfolio_value`).
2. Leave a cash buffer. Deployable capital is:

   `portfolio_value * (1 - cash_reserve)`

3. Cap the book at `max_positions` names. Rank the signal, take at most that
   many, and equal-weight the deployable capital across the names you actually
   hold this bar (`n = min(max_positions, len(selected))`).
4. Target weight per name is `(1 - cash_reserve) / n`. Call
   `order_target_percent(symbol, weight)` for every name in the target set.
5. **No leverage.** The sum of target percents must stay `<= 1 - cash_reserve`.
   Ten names at `0.1` with `cash_reserve = 0` is fully invested, not a reserve.
6. **No fixed share counts.** `order_target(symbol, 10)` is the anti-pattern.
   `order_target(symbol, 0)` is allowed only to flatten.

`cash_reserve` and `max_positions` are required risk parameters. Typical
defaults: `cash_reserve=0.05` (or `0.10`), `max_positions=10`.

## Rebalance every bar

On every bar, after the signal set is known:

- **Rebalance keepers** toward the current equal-weight target, including names
  already in `portfolio.positions`. Skipping `if symbol in portfolio.positions`
  freezes stale share counts while prices (and `n`) move.
- **Flatten leavers** with `order_target(symbol, 0)` for any held name that is
  not in the target set this bar (dropped from the rank, failed the signal, or
  no longer in the universe).

Do not buy once and hold a fixed lot. Do not only order new entries.

## Parameterize lookbacks and risk

`Algorithm` / `Function` treat extra callable arguments (anything that is not
`Asset`, `Assets`, or `Portfolio`) as `FunctionParameter`s with typed defaults.
Workers apply them through `Algorithm.configure`.

Generated strategies must expose at least:

- indicator lookbacks (SMA windows, MACD `fastperiod` / `slowperiod` /
  `signalperiod`, RSI `timeperiod` and oversold threshold)
- `cash_reserve`
- `max_positions`

Hardcoded `tail(10)` / `tail(30)`, `head(10)`, `0.1`, or `timeperiod=14` are
not acceptable as the only knobs.

## Portfolio API (do not invent another)

`foreverbull.models.finance.Portfolio` is the sizer:

| Method | Use |
| --- | --- |
| `order_target_percent(symbol, percent)` | Size and rebalance. `percent` is a fraction of `portfolio_value`. |
| `order_target(symbol, 0)` | Flatten a leaver. |
| `order_target(symbol, 10)` | **Forbidden** as a position size. |
| `order` / `order_percent` / `order_value` | Delta orders; templates should not use these for target book construction. |
| `deployable_value(cash_reserve)` | `portfolio_value * (1 - cash_reserve)`. |
| `equal_weight_percent(n_selected, cash_reserve, max_positions)` | Per-name target: `(1 - cash_reserve) / n`. |

Percent sizing is `portfolio_value * percent` then `int(value / close)`.
Protobuf `Portfolio` also carries `cash`; templates still size from
`portfolio_value` and `cash_reserve`, not from leftover cash after a previous
bar's mistakes.

Do not rewrite the `order_*` API for this contract. Per-name weights come from
`deployable_value` and `equal_weight_percent` (on `Portfolio`, and in
`allocation.py` copied next to each strategy). Integer shares still go through
`order_target_percent`.

## Status

Generated strategies are backtests, not fixed-lot demos. They must follow the
contract above. Do not describe or implement a buy as a magic share count.

### Simple — `simple/{{cookiecutter.project_slug}}/{{cookiecutter.project_slug}}/crossover.py`

Pandas SMA crossover (no TA-Lib). Long when the short window is above the long
window; drop the name when it is not. Size with `order_target_percent` from
`equal_weight_percent`, leave `cash_reserve`, cap at `max_positions`, rebalance
keepers every bar, and flatten leavers with `order_target(symbol, 0)`. Lookbacks
(`short_window` / `long_window`) and the risk knobs are `Function` parameters.

The checked-in file still uses `order_target(symbol, 10)` instead of that
contract: no percent targets, no rebalance of keepers, hardcoded `tail(10)` /
`tail(30)`, no `cash_reserve` or `max_positions`. Generated `tests/capital_test.py`
fails until `crossover.py` is sized. Keep this template pandas-only.

### Advanced — `macd.py` and `rsi.py`

Meets the contract. Both leave `cash_reserve` (default `0.05`), cap at
`max_positions` (default `10`), equal-weight deployable capital across the
names that qualify this bar, rebalance keepers every bar (including names
already held), and flatten leavers with `order_target(symbol, 0)`.

RSI ranks oversold names (`rsi < oversold`). If fewer than `max_positions`
qualify, remaining deployable is concentrated across those names. MACD ranks
positive MACD the same way. Lookbacks and risk knobs are `Function` defaults:
RSI `timeperiod` / `oversold`, MACD `fastperiod` / `slowperiod` /
`signalperiod`, plus `max_positions` and `cash_reserve`. Advanced stays on
TA-Lib; simple must not.

## Out of scope for these templates

Stooq / Yahoo replacement, ETF universe / ISIN identity, strategy-domain RPCs,
live trading, and Kind / Helm wiring. Do not enable live trading from a
generated project.
