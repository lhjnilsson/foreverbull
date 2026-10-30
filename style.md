# Style

The design order is **safety, then performance, then developer experience**. A change that is faster or shorter and can persist a wrong share count, a wrong price, or a wrong identity is a regression.

Read this file before editing `cmd/`, `pkg/`, `internal/`, or `client/`. Domain contracts still name behavior and fields. When this file and a contract disagree about a field or a caller, the contract wins:

- [pkg/backtest/universe.md](pkg/backtest/universe.md) — ISIN identity, point-in-time membership, bars per provider
- [pkg/finance/profiles.md](pkg/finance/profiles.md) — price source on the backtest, credentials only in the profiles file
- [pkg/finance/trading.md](pkg/finance/trading.md) — `supplier.Trading`, signed share amount, paper only
- [pkg/strategy/implementation.md](pkg/strategy/implementation.md) and [pkg/strategy/research-to-paper.md](pkg/strategy/research-to-paper.md)
- [pkg/finance/broker-door.md](pkg/finance/broker-door.md) — broker-mediated OHLC / positions / results; no client DB URL

Apply this file to the code a task already touches. A style mismatch in untouched code is not a task. A reading of this file that would change a stored price, a share count, or an order a user already gets needs confirmation before the code changes.

Generated protobuf (`pkg/pb/`, `internal/stream/pb/`, `client/src/foreverbull/pb/`) and mockery output (`mock_*.go`) stay generated. This file does not govern the Grafana plugin's TypeScript or Helm templates, except that a trading or order default is written down in Go or Python and rejected when it is the protobuf zero.

## Commit messages

> **Every commit subject starts with one [gitmoji](https://gitmoji.dev).** The emoji is the intent. Then `type(scope): outcome`. The body says why.

```
🐛 fix(test): keep Python unit tests off Postgres 18 and port 7878

postgres:alpine is Postgres 18 and exits when data is mounted at
/var/lib/postgresql/data, so every SQL test failed setup. Pin the
fixture to 17-alpine. Worker configs also dialed 7878 while the
namespace fake listens on a random port.
```

Write the emoji character, not the `:shortcode:`. One emoji, the one whose gitmoji description matches the change. Pick it from the table. When the change is more specific than the type, use the narrower gitmoji and keep the type: a red CI build is `💚 fix(ci): …`, a linter finding is `🚨 fix(client): …`. Do not stack emojis. Do not invent one. Joke entries on the gitmoji list (`:poop:`, `:beers:`, `:clown_face:`) are not commit intents.

| Emoji | Gitmoji | Subject |
| --- | --- | --- |
| ✨ | `:sparkles:` | `feat` — new behavior a caller can observe |
| 🐛 | `:bug:` | `fix` — a correction to existing behavior |
| 🩹 | `:adhesive_bandage:` | `fix` — a small, non-critical correction |
| ⚡️ | `:zap:` | `perf` — same prices, share counts, and portfolio values, less work |
| 📝 | `:memo:` | `docs` — contracts or docs only |
| ✅ | `:white_check_mark:` | `test` — add or update tests |
| 🧪 | `:test_tube:` | `test` — a failing test added first |
| ♻️ | `:recycle:` | refactor — behavior unchanged |
| 💚 | `:green_heart:` | `fix(ci)` — the build was red |
| 👷 | `:construction_worker:` | CI added or changed |
| 🚨 | `:rotating_light:` | linter or compiler findings |
| 🔧 | `:wrench:` | `chore` — configuration |
| 📌 | `:pushpin:` | pin a dependency |
| ⬆️ | `:arrow_up:` | upgrade a dependency |
| ➕ | `:heavy_plus_sign:` | add a dependency |
| ➖ | `:heavy_minus_sign:` | remove a dependency |
| 🙈 | `:see_no_evil:` | `.gitignore` |
| 🗃️ | `:card_file_box:` | schema change |
| 🧱 | `:bricks:` | Helm, Kind, or other infrastructure |
| 🔒️ | `:lock:` | security or privacy |
| 🔥 | `:fire:` | remove code or files |
| ⚰️ | `:coffin:` | remove dead code |
| 🚚 | `:truck:` | move or rename |
| 🎨 | `:art:` | format only |
| 💥 | `:boom:` | breaking change |
| ⏪️ | `:rewind:` | revert |

**Subject.** Imperative, lowercase after the colon, no trailing period. Keep the text after the emoji to about 72 characters so `git log --oneline` stays one line. The scope is the area a reader would search: `finance`, `backtest`, `strategy`, `cli`, `client`, `grafana`, `deploy`, `testing`, `templates`, `ci`. Leave the scope off only when the change is repo-wide (`📝 docs: record paper trading provider defaults`).

**Body.** A blank line after the subject, wrapped at 72 characters. Say the constraint that forced the change: which input failed, which number must stay the same, which default is now explicit. A `perf` body names the result that did not move. A `docs` body names what is now locked. Skip the body when the subject already contains that reason and fits on one line. The pull-request text is not in `git blame`.

**One change.** A behavior change and the contract that freezes it are neighbors, not the same commit. Leave generated cookiecutter project trees out. Regenerated protobuf belongs in the commit that changed the `.proto`.

**Tidy history.** Pull requests merge with Rebase and merge only, so every commit of a PR lands on `main` as written. By the time a PR leaves draft, each commit is one intentional change with a gitmoji subject. No `fixup!`, `squash!`, or "address review" commits. While the PR is in draft and under review, `git commit --fixup <sha>` is fine. Before you say the PR is ready, fold them in with `git rebase -i --autosquash <base>` and `git push --force-with-lease`. A PR doesn't leave draft until then.

### 📝 In documentation

Use the same emoji in Markdown, and use it once. The table above is the catalog, so this page may list them, and a sentence that shows a commit subject includes that subject's emoji. Every other page gets one emoji, at the start of the heading or callout a reader would be wrong to skip: 📝 on a locked contract, 🔒️ where credentials or a live-trading refusal sit, 💥 on a break, ✅ on a check that must pass. Paragraphs, tables, and code stay plain. A heading that is already specific needs no emoji. Do not walk an existing document and decorate it. When you edit a page, add the emoji only if that page does not already have one.

## House rules

- Early returns on errors. One decision per branch. A switch on order type, time in force, or provider lists every known case and a default that fails.
- Every loop over external input has a bound: the slice length, a context cancellation, a page cap, or a byte cap.
- Check the value where an order, price, share count, ISIN, or date range enters or leaves. Programmer errors crash. Operating errors return. A quota of checks per function only adds filler.
- Validate before a write and again after the read, on both sides of a process, disk, or broker boundary.
- Test the valid order and the order that must be refused.
- Ports, body sizes, queue reads, and in-memory caches have a ceiling written next to the loop. The process allocates per request. Growth that comes from market or user input has a bound.
- Network, then disk, then memory, then CPU. Batch the slow resource.
- ISIN, symbol, amount, and provider mean what the contracts say. Units live in the type or at the end of a raw number.
- A comment is a sentence about a constraint. `TODO` does not land.
- **A commit subject starts with one gitmoji.** The rule and the table are under [Commit messages](#commit-messages). A doc page uses that same emoji once, on the line a reader must not miss.
- Go uses `gofmt` (tabs) and MixedCaps. Python uses Ruff 0.17.0, 4 spaces, and 120 columns (`client/pyproject.toml`).
- Go and Python return early on the failing path. Write both branches when both outcomes are domain results (filled and rejected, in the universe and dropped).
- A new function should fit on one screen, about 70 lines. Do not split a function only to hit the number.
- Share counts are the protobuf `int32`. Prices and portfolio values are `float64` because that is the stored type. Sizes that cross the wire name their width. `int` is fine for a local index.
- Use the standard library and the stack already in `go.mod` and `client/pyproject.toml`. A new module needs a reason those cannot do the job.
- The backtest and the unit test are the check. A fuzzer can be added for a parser. A green test does not replace the check in the function.

## Safety

### Control flow

Keep the decision in the parent and the arithmetic in a helper. [Push `if`s up and `for`s down](https://matklad.github.io/2023/11/15/push-ifs-up-and-fors-down.html). A helper that places an order, writes a bar, or mutates a portfolio returns the value; the caller decides what to do with it.

Prefer a sequence of single checks that each return their own error. `AlpacaClient.orderRequest` is the shape: nil order, empty ISIN, unknown ISIN, zero amount, `math.MinInt32`, unspecified type, and a non-positive limit price are separate failures. A compound boolean hides which clause failed.

Write the condition that holds. For an index and a count:

```go
if index < count {
    // in range
} else {
    return fmt.Errorf("index %d: %w", index, errOutOfRange)
}
```

`if index >= count` forces the reader to negate the invariant. Early `if err != nil` stays; that is the operating-error path, and the positive event is the code after it.

Recursion is for a tree whose depth is the schema (a fixed config or protobuf structure). Holdings, message chains, directory walks, and retry loops are iterative and bounded. There is no direct recursion on a per-bar or per-order path.

`unsafe`, cgo, and `ctypes` are out. Generated protobuf already has its own runtime.

### Bounds

A `for _, item := range items` is bounded by `items`. A `for {` on a request path, a broker poll, a holdings download, or a socket read has a second bound: `ctx.Done()`, a max page count, or a max byte count. `io.LimitReader(resp.Body, 1<<20)` in the Alpaca client is the pattern for an HTTP body.

Name the ceiling next to the loop as a constant (`alpacaOpenOrderLimit`, `BACKTEST_PORT_RANGE_START` / `END`). A port range is `27000`–`27015`. Do not scan past it.

An event loop that lives for the process (a worker run, a NATS subscriber) documents that it runs until cancellation, and it still selects on the context or the stop event. A missing bar, a broker timeout, or a closed socket exits that iteration with an error. It does not spin.

Preallocate when the count is known: `make([]*pb.Order, 0, len(raws))`. A map that caches bars or listings for one run is discarded with the run. A cache that lives for the process has a max number of entries.

### Programmer errors and operating errors

A broken invariant crashes. An expected failure is handled.

**Programmer errors** mean the process is about to persist a lie: a share count that overflowed `int32`, a negative price stored as a close, a portfolio whose positions were silently dropped. In Go, `panic` after the check is the crash. In Python, raise an exception that the worker does not catch and report as success. `assert` is not that check. Python strips `assert` when it runs with `-O`.

**Operating errors** are expected. The broker refused the order, the bar is missing, the context is canceled, the YAML is invalid, the ISIN is unknown. Go returns a wrapped sentinel (`supplier.ErrRejected`, `supplier.ErrNotTradable`, `supplier.ErrUnmappedPosition`, `ctx.Err()`). Python raises a typed exception (`exceptions.ConfigurationError` and the finance errors already in the client). The caller handles it. `_ = err`, `except Exception: pass`, and a log line with no return are how a refused order becomes a filled one.

A canceled context returns `ctx.Err()` and does no HTTP and no insert. That is already the trading contract.

Startup is the place for fatal configuration: `TRADING_ENABLED=true` with a missing profiles file, a port that is not a number. After startup, a bad request fails the request.

### Both sides of a boundary

For a property that must hold, check it in at least two places that can fail independently:

- before the broker request and again when the response is decoded (`orderRequest`, then `mapOrder`)
- before an `INSERT` and again when a later query scans the row
- in the adapter and in the wrapper that resolves a symbol to an ISIN (`trading.md`)

`mapOrder` refuses a fill whose average price is missing. That is the negative space of "a non-zero `filled_amount` has an `average_price`". Dropping an unmapped position is forbidden; `GetPortfolio` fails the whole snapshot with `ErrUnmappedPosition`.

Split checks. `if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0)` is one idea (the limit is a real positive price) and can stay one condition. Two unrelated ideas (empty ISIN, zero amount) are two `if` statements so the error says which one failed.

A comment that says "this cannot happen" can be the check instead, when the condition is cheap. The check runs in production. The comment does not.

### Positive and negative space

A function that builds an order, a bar, a portfolio, or a universe states what it accepts and what it refuses.

Refuse the protobuf zero when it is not a real choice. `ORDER_TYPE_UNSPECIFIED` is not a market order. Amount `0` is not a hold. An empty ISIN is not resolved by guessing. An omitted backtest provider becomes `yahoo` only where the code assigns `yahoo` and the caller can see that assignment.

`math.MinInt32` cannot be negated into a positive quantity. Reject it before the sign flip, as `orderRequest` does.

### Index, count, and size

These are different integers. An index is 0-based. A count is how many. A size is bytes or a stride. The last valid index is `count - 1`. Turning an index into a count adds one. Turning a count into a size multiplies by the element width.

Name them so the conversion is visible: `barIndex`, `symbolCount`, `bodySize`. A loop variable `i` is fine. A domain variable called `n` or `len` is not, once it leaves the loop.

Calculate the value on the line that uses it. A share count computed at the top of a function and applied after a portfolio update is a place-of-check to place-of-use bug.

### Shares, prices, and division

Share amounts are integers. The platform order is a signed `int32`: positive buys, negative sells, zero is rejected. Algorithms emit a symbol plus that signed amount. Adapters translate; they do not change the share count except for the sign they send the broker.

Prices, OHLC, and portfolio value are `float64`. That is the column type and the protobuf type. Do not introduce a decimal type as a drive-by. Do not compare a price with `==` in production. Reject `NaN` and both infinities. A limit price is greater than zero. Tests that compare floats use a stated delta (`require.InDelta`).

Say how you round. Python `int(value / price)` truncates toward zero, so `int(-1.9)` is `-1`, not `-2`. Go integer division truncates toward zero as well. If the order must not spend a fractional share, truncation toward zero is the operation, and the name or a one-line comment says so. If the operation is floor, call `math.Floor` and test a negative input. Name a helper `divTrunc` or `divFloor` when `a / b` does not show which rounding you mean.

`equal_weight_percent` divides deployable capital by the number of names. The divisor is at least 1 before the division. A cash reserve outside `[0, 1)` is rejected in new code rather than producing a weight above 1 or a negative weight.

Formatting a price for a broker states the format. `strconv.FormatFloat(price, 'f', -1, 64)` is explicit. A default format argument is not.

### Secrets and leftover bytes

Profiles, API keys, and tokens stay in the profiles file and the Kubernetes Secret. They are not logged, not written into Postgres, and not copied into an error string. A struct that held a credential is not reused for another request without being replaced.

A buffer you send or store is filled up to a counted length, and the unused tail is not part of the message. `LimitReader` plus decoding into a struct is the usual shape. Do not reslice a credential buffer and pass the spare capacity onward.

### Say why

A comment says why the code is this way: a broker field that must be absent, a snapshot date that fills a gap, a port that is also a Service name. It is a sentence. It is not a narration of the next line.

`golangci-lint` runs `godox`, which fails the build on `TODO`, `FIXME`, and `BUG`. An unfinished path returns an error or it is finished. A note to a future reader is a sentence without those words.

## Performance

Sketch the slow part before writing it. The order of cost, after you account for how often the work runs, is network, then disk, then memory, then CPU. A per-symbol query inside a bar loop dominates a tight CPU loop. The faster-execution work exists because of that.

Batch the slow resource. One SQL statement for many bars beats one statement per symbol. One broker request per order stays one request; do not hide a fan-out inside a mapper. Control-plane work (resolve the listing, open the engine, read config) happens before the data plane (walk bars, emit orders).

A hot loop takes the values it needs as arguments. It does not look up config, open a connection, or allocate a client on each element. Pass the provider, the symbol, and the row. The caller owns the connection.

Be predictable. Sort or group symbols once, then scan. A branch per row that could have been a split into two slices is paid on every bar.

## Developer experience

### Names

Go is MixedCaps. Exported initialisms in hand-written Go are `ISIN`, `OHLC`, `URL`, `ID`, `HTTP`. Generated getters stay as generated (`GetIsin`). Python is `snake_case`. A Python function says `ohlc_provider`, and a constant may say `OHLC`.

Use the domain noun. `symbol` is the ticker alias. `isin` is the identity. `amount` is the signed share delta. `provider` is `yahoo`, `stooq`, or `alpaca`. Do not invent a second word for the same thing.

Do not abbreviate a domain word in new code (`message`, `connection`, `portfolio`, `execution`). Keep the short names Go and Python already use for mechanics: `ctx`, `err`, `ok`, `db`, `req`, `i`.

`time.Duration` and `time.Time` carry the unit. A raw integer gets the unit at the end: `recvTimeoutMS`, `timeout_ms`. When the type is `time.Duration`, Go keeps `MaxLatency`.

A function with two bare integers that can be swapped takes a small struct or named parameters so the call site cannot trade a count for a size. Python keyword-only arguments do the same job.

`ctx` is the first Go parameter. Callbacks and handlers go last when a function takes one.

### Shape of a function

A function that fits on one screen can be checked. New functions aim for that, about 70 lines, with a few parameters and a single result (value plus error, or a raised exception). Do not reshape a working function only to pass a length check.

Declare a variable at the use site. Do not keep a share count, a listing, or a price alive across a block that no longer needs it.

Return the simplest type that is honest. `error` alone beats `(bool, error)` when the bool repeats the error. A pointer result is nil only together with a non-nil error, or the contract says an empty portfolio is a real empty portfolio.

### Comments

Explain a non-obvious constraint. Skip the comment when the name and the type already say it. Commit subjects and bodies are [Commit messages](#commit-messages).

### Formatters and linters

Run the formatter. Do not hand-align past what it does.

| Language | Tool |
| --- | --- |
| Go | `gofmt` / `goimports` (enabled in `.golangci.yaml`). `gofmt` does not wrap lines; break a signature when it is hard to read, with a trailing comma. |
| Python | `task client:lint` and `task client:format` (Ruff **0.17.0**, line length 120). Pyright runs in the commit hook. |

Go lint enabled for this repo is `exhaustive`, `forbidigo`, `godox`, and `testifylint`. `exhaustive` is why a new enum switch needs every case. Generated files are excluded. Ruff ignores `client/src/foreverbull/pb/`, cookiecutter templates, and the local `simple_foreverbull` / `advanced_foreverbull` trees.

Pass library options by name at the call site when the default would change the meaning. `block_on_dial=True` and an explicit receive timeout on a socket are visible. `yaml` and protobuf zeros are not a trading policy.

### Dependencies

Reach for the standard library, then for a module the repo already vendors: `pgx`, gRPC, NATS, the Kubernetes client, SQLAlchemy, `pynng`. Add a module only when those cannot express the job, and say why in the change. Do not add a dependency to avoid a twenty-line function.

## Tests

A test names the invariant in the first lines or the case name: the order it builds, the bar it inserts, the error it expects.

Cover both sides. The valid ISIN that maps to one listing, and the empty ISIN, the ambiguous symbol, the zero amount, `MinInt32`, the `NaN` limit, the unmapped position. The missing close is an operating error, not a skipped order.

Share counts are exact integers. Prices in a test use an explicit delta. Do not loosen a delta to make a new rounding mode pass.

Table-driven Go tests are the default for a pure function with several inputs. `testifylint` is on; use the testify assertion that matches the check (`require.ErrorIs`, `require.Equal`, `require.InDelta`).

A backtest integration test is for a path that needs Zipline, Postgres, NATS, or Minio. A pure share-count or ISIN rule is a unit test with no container. Do not move a unit test onto Kind.

## Before you finish a change

1. The order, price, or ISIN path you touched rejects the empty and the zero case, and checks the value again after a read from disk or a broker.
2. Every new loop over external input has a bound you can point at.
3. Operating failures return. A `panic` or an uncaught exception is reserved for a broken invariant.
4. Rounding of a share count is written down and tested, including a negative amount when the function can see one.
5. `gofmt` / `goimports` or Ruff has been run on the files you edited. No `TODO`.
6. You did not restyle a file the task did not need to change.
7. A commit you create starts with one gitmoji from [Commit messages](#commit-messages), then `type(scope): outcome`, and the body says why.
8. A doc page you edited has at most one gitmoji, on the line a reader must not miss.
