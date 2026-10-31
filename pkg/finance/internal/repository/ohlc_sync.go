package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// ErrNoBars matches a NoBarsError.
var ErrNoBars = errors.New("no price data")

// NoBarsError is a fetch that had to return bars and returned none. It is an
// error so no caller reports success after storing nothing. Its text is the
// same sentence backtest create shows when a provider returns no prices: it
// names the provider and the dates and suggests what to try next. The
// DownloadHistoricalData RPC still reports it to backtest create with its own
// "no price data for SYM" status text.
type NoBarsError struct {
	Provider string
	Symbol   string
	Start    time.Time
	End      *time.Time
}

func (e *NoBarsError) Error() string {
	end := "today"
	if e.End != nil {
		end = tradingDay(*e.End)
	}
	return fmt.Sprintf("%s returned no prices for %s between %s and %s. "+
		"Check that %s traded between those dates, try again later, or use another provider with --provider.",
		priceProvider(e.Provider), e.Symbol, tradingDay(e.Start), end, e.Symbol)
}

func (e *NoBarsError) Unwrap() error { return ErrNoBars }

// IssuesURL is where the finance service's user-facing errors ask for a
// report.
const IssuesURL = "https://github.com/lhjnilsson/foreverbull/issues"

// StoreError is a database failure while storing one symbol's prices. Its
// text is a fixed sentence, so no SQL or driver detail reaches a task state
// or a client. Err keeps the cause for the server log.
type StoreError struct {
	Symbol string
	Err    error
}

func (e *StoreError) Error() string {
	return fmt.Sprintf("Couldn't store prices for %s. Try again in a few minutes. "+
		"If it keeps failing, report it at %s.", e.Symbol, IssuesURL)
}

func (e *StoreError) Unwrap() error { return e.Err }

// FetchBars returns the provider's bars for one symbol from start through end
// (nil end: through today).
type FetchBars func(start time.Time, end *time.Time) ([]*financepb.OHLC, error)

// syncLockTimeout is how long Sync waits for the symbol lock before it fails.
// syncIdleTimeout is how long its transaction may stay idle, for example while
// fetch waits on a provider (Yahoo requests time out after 30s). Both are
// Postgres interval strings; tests shorten them.
var (
	syncLockTimeout = "60s"
	syncIdleTimeout = "90s"
)

// rescaleTolerance is how far the stored and fetched close of the overlap day
// may differ before the stored history is treated as being on another scale,
// for example after a split.
const rescaleTolerance = 0.005

// Sync brings the stored history of provider and symbol up to start through
// end. Ingest and the DownloadHistoricalData RPC both store bars through it,
// so both re-scale after a split the same way.
//
// The provider is asked first, outside any transaction: the stored range and
// overlap close are read without the lock, and fetch is called for every span
// that plan needs. Only then does Sync open one transaction whose first
// statement takes pg_advisory_xact_lock(hashtext('ohlc:' || symbol)), re-read
// the stored range and overlap close, and make the append-or-replace decision
// again under that lock, so two syncs of one symbol still store one after the
// other and can't leave days on two scales. When the locked decision asks for
// the spans already fetched, their bars are stored and no provider call runs
// under the lock. Only when another sync changed the stored history in
// between, so the decision now needs a different span, is fetch called again,
// while the lock is held. A slow provider therefore holds neither the lock
// nor a pooled connection in the normal case.
//
// Nothing stored yet: start through end is fetched and inserted.
//
// Stored history that starts on or before start: the fetch begins on the last
// stored day, so it overlaps one stored day. When that day comes back with a
// close within rescaleTolerance of the stored one, the new days are upserted.
// Otherwise (the close moved, the day is missing from the fetch, or either
// close is not a positive finite price) the scale of the stored history can't
// be trusted, so the whole range from the first stored day is fetched again
// and replaced.
//
// Stored history that starts after start: start through the last stored day
// (or end, when later) is fetched and replaced, so no stored day is left on an
// old scale.
//
// fetch errors come back unchanged. A refused bar is an InvalidPriceError and
// a refetch with no bars is a NoBarsError; in both cases the stored rows stay
// as they were. Any database error is a StoreError.
//
// The transaction covers ohlc rows only. Callers write the asset row, and
// for a later ticker the asset_symbol alias, before Sync; those stay when
// the bars roll back.
func (db *OHLC) Sync(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	fetch FetchBars,
) error {
	begin, ok := db.Conn.(txBeginner)
	if !ok {
		return &StoreError{Symbol: symbol, Err: ErrNoTransaction}
	}
	fetched := &fetchCache{fetch: fetch}
	if err := db.classify(symbol, db.decide(ctx, provider, symbol, start, end, fetched.get, false)); err != nil {
		return err
	}
	if pool, ok := db.Conn.(*pgxpool.Pool); ok {
		release, err := acquireSyncSlot(ctx, pool)
		if err != nil {
			return &StoreError{Symbol: symbol, Err: err}
		}
		defer release()
	}
	tx, err := begin.Begin(ctx)
	if err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	// Bound how long this transaction waits for the symbol lock, and how long
	// it may sit idle (a fetch runs while it is open when the locked decision
	// changed), so a stuck sync gives the lock and its connection back. set_config(..., true) is SET LOCAL: both
	// reset when the transaction ends.
	if _, err := tx.Exec(ctx,
		`SELECT set_config('lock_timeout', $1, true), set_config('idle_in_transaction_session_timeout', $2, true)`,
		syncLockTimeout, syncIdleTimeout); err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	// The lock key is the symbol only, not provider and symbol: syncs of one
	// symbol from two providers wait for each other too. That is more
	// serialisation than the per-provider rows need, but never less, and it
	// keeps one key per instrument. hashtext can collide, which only makes
	// two unrelated symbols wait for each other.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || $1))`, symbol); err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	inTx := &OHLC{Conn: tx}
	if err := inTx.classify(symbol, inTx.decide(ctx, provider, symbol, start, end, fetched.get, true)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	return nil
}

// syncSlots holds one semaphore per pool. Each Sync keeps a pooled
// connection for its whole transaction, fetch included, so without a cap a
// burst of downloads (10 RPC workers plus ingest) could take every connection
// and starve the server's health Ping.
var syncSlots sync.Map // *pgxpool.Pool -> chan struct{}

// syncSlotsFor is the semaphore for pool: MaxConns-1 slots, clamped to at
// least 1. From MaxConns 2 up, one connection always stays free for Ping and
// other callers. With MaxConns 2 that means one Sync at a time and one free
// connection. With MaxConns 1 the clamp still allows one Sync, which then
// holds the pool's only connection, so no connection is kept free: a Ping or
// any other query on that pool waits until the Sync's transaction ends.
func syncSlotsFor(pool *pgxpool.Pool) chan struct{} {
	if slots, ok := syncSlots.Load(pool); ok {
		return slots.(chan struct{}) //nolint:forcetypeassert // only chan struct{} is stored
	}
	n := int(pool.Config().MaxConns) - 1
	if n < 1 {
		n = 1
	}
	slots, _ := syncSlots.LoadOrStore(pool, make(chan struct{}, n))
	return slots.(chan struct{}) //nolint:forcetypeassert // only chan struct{} is stored
}

// acquireSyncSlot waits for a free Sync slot on pool or for ctx to end.
func acquireSyncSlot(ctx context.Context, pool *pgxpool.Pool) (func(), error) {
	slots := syncSlotsFor(pool)
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// fetchError marks an error that came from fetch, so Sync returns it as is.
type fetchError struct{ err error }

func (e *fetchError) Error() string { return e.err.Error() }
func (e *fetchError) Unwrap() error { return e.err }

// fetchCache serves each span Sync fetched before taking the lock. A span is
// the trading day of from and of to (nil: through today). Any other span goes
// to fetch and is kept too.
type fetchCache struct {
	fetch   FetchBars
	entries []fetchedSpan
}

type fetchedSpan struct {
	from, to string
	bars     []*financepb.OHLC
}

func spanKey(from time.Time, to *time.Time) (string, string) {
	if to == nil {
		return tradingDay(from), ""
	}
	return tradingDay(from), tradingDay(*to)
}

func (c *fetchCache) get(from time.Time, to *time.Time) ([]*financepb.OHLC, error) {
	f, t := spanKey(from, to)
	for _, e := range c.entries {
		if e.from == f && e.to == t {
			return e.bars, nil
		}
	}
	bars, err := c.fetch(from, to)
	if err != nil {
		return nil, err
	}
	c.entries = append(c.entries, fetchedSpan{from: f, to: t, bars: bars})
	return bars, nil
}

// classify returns fetch errors unchanged, keeps a refused bar, a missing
// refetch and a StoreError as they are, and turns anything else into a
// StoreError.
func (db *OHLC) classify(symbol string, err error) error {
	if err == nil {
		return nil
	}
	var fetched *fetchError
	if errors.As(err, &fetched) {
		return fetched.err
	}
	var invalid *InvalidPriceError
	var noBars *NoBarsError
	var stored *StoreError
	if errors.As(err, &invalid) || errors.As(err, &noBars) || errors.As(err, &stored) {
		return err
	}
	return &StoreError{Symbol: symbol, Err: err}
}

// decide makes the append-or-replace decision for start through end and
// fetches what it needs. With apply false it only fetches, so the bars are
// ready before the lock is taken; with apply true it stores them too.
func (db *OHLC) decide(ctx context.Context, provider, symbol string,
	start time.Time, end *time.Time, fetch FetchBars, apply bool,
) error {
	storeBars := func(bars []*financepb.OHLC) error {
		if !apply {
			return nil
		}
		return db.StoreBars(ctx, provider, symbol, bars)
	}
	refetchAndReplace := func(from time.Time, to *time.Time) error {
		bars, err := fetch(from, to)
		if err != nil {
			return &fetchError{err: err}
		}
		if !apply {
			return nil
		}
		// The batch is validated first, so a bad bar never turns into a
		// missing day.
		return db.ReplaceRange(ctx, provider, symbol, from, to, bars)
	}

	first, last, stored, err := db.Range(ctx, provider, symbol)
	if err != nil {
		return err
	}

	if !stored {
		bars, err := fetch(start, end)
		if err != nil {
			return &fetchError{err: err}
		}
		return storeBars(bars)
	}

	if start.Before(first) {
		spanEnd := end
		if end != nil && end.Before(last) {
			spanEnd = &last
		}
		return refetchAndReplace(start, spanEnd)
	}

	if end != nil && !end.After(last) {
		// The requested range is already stored.
		return nil
	}

	bars, err := fetch(last, end)
	if err != nil {
		return &fetchError{err: err}
	}
	matches, err := db.overlapMatches(ctx, provider, symbol, last, bars)
	if err != nil {
		return err
	}
	if !matches {
		return refetchAndReplace(first, end)
	}
	return storeBars(bars)
}

// overlapMatches reports whether the fetched close for the overlap day is a
// positive finite price within rescaleTolerance of the stored close. A missing
// overlap day or a bad close on either side is not a match, so the caller
// replaces the stored history instead of appending next to it.
func (db *OHLC) overlapMatches(ctx context.Context, provider, symbol string, day time.Time,
	fetched []*financepb.OHLC,
) (bool, error) {
	want := tradingDay(day)
	for _, o := range fetched {
		if tradingDay(o.GetTimestamp().AsTime()) != want {
			continue
		}
		if !positivePrice(o.GetClose()) {
			return false, nil
		}
		storedClose, err := db.Close(ctx, provider, symbol, day)
		if err != nil {
			return false, err
		}
		if !positivePrice(storedClose) {
			return false, nil
		}
		return math.Abs(o.GetClose()-storedClose)/storedClose <= rescaleTolerance, nil
	}
	return false, nil
}
