package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
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
// Stored history that starts on or before start: one fetch through end. With
// no weekday missing from start up to the last stored day it begins on the
// last stored day. With one missing it begins on the closest stored day
// before the first missing weekday (or on that weekday when no earlier day
// is stored from the scan start on). Only days from start, and no more than
// gapScanDays before the last stored day, are looked at, so an older gap
// stays as it is and backtest create fails on it (the missing-session
// check). Every check day, the last stored day and the stored day next to
// the gap, is inside the fetched span. When each comes back with a
// close within rescaleTolerance of the stored one, the bars for missing days
// are inserted, the last stored day is upserted as before, and every other
// stored row is left as it is.
// Otherwise (a close moved, a check day is missing from the fetch, or either
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
	return db.sync(ctx, provider, symbol, start, end, fetch, nil, nil)
}

// FetchDividends returns the provider's cash dividends with an ex-date from
// the day of start through the day of end (nil end: through today).
type FetchDividends func(start time.Time, end *time.Time) ([]supplier.Dividend, error)

// ErrStooqCashDividends refuses cash dividends on stooq, whose bars already
// include them: booking the cash too would count each dividend twice. No
// user flow reaches it, because SyncSource stores stooq through Sync, so it
// means foreverbull itself went wrong.
var ErrStooqCashDividends = errors.New( //nolint:staticcheck // ST1005: agreed user-facing copy ends with a period
	"foreverbull was asked to store cash dividends for stooq, whose prices already include them, " +
		"so they'd be counted twice. Nothing was stored. " +
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")

// SyncWithDividends is Sync for a provider whose bars are split-adjusted but
// not dividend-adjusted (yahoo, alpaca). In the same locked transaction, after
// the bars, it replaces the dividends of the symbol's whole stored span and
// records that span as fetched. A refetch that re-scaled the bars therefore
// re-scales the dividends too: the two never sit on different split bases.
//
// Dividends are fetched before the lock, from the earlier of start and the
// first stored day through today. When the stored span under the lock starts
// earlier still (another sync stored older bars in between), they are fetched
// again while the lock is held. A failed fetch comes back as the fetch
// returned it, and a refused dividend as a DividendError; either way nothing
// is stored. On stooq it is ErrStooqCashDividends, and nothing is fetched or
// stored.
func (db *OHLC) SyncWithDividends(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	fetch FetchBars, fetchDividends FetchDividends,
) error {
	if priceProvider(provider) == "stooq" {
		return ErrStooqCashDividends
	}
	if fetchDividends == nil {
		return &StoreError{Symbol: symbol, Err: errors.New("no dividend fetch")}
	}
	return db.sync(ctx, provider, symbol, start, end, fetch, fetchDividends, nil)
}

func (db *OHLC) sync(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	fetch FetchBars, fetchDividends FetchDividends, then func(context.Context, pgx.Tx) error,
) error {
	begin, ok := db.Conn.(txBeginner)
	if !ok {
		return &StoreError{Symbol: symbol, Err: ErrNoTransaction}
	}
	fetched := &fetchCache{fetch: fetch}
	if err := db.classify(symbol, db.decide(ctx, provider, symbol, start, end, fetched.get, false)); err != nil {
		return err
	}
	var dividends *dividendCache
	if fetchDividends != nil {
		var err error
		dividends, err = db.prefetchDividends(ctx, provider, symbol, start, fetchDividends)
		if err != nil {
			return err
		}
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
	if dividends != nil {
		if err := inTx.replaceDividends(ctx, provider, symbol, dividends); err != nil {
			return err
		}
	}
	if then != nil {
		if err := then(ctx, tx); err != nil {
			return err
		}
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
	var duplicate *DuplicateDayError
	var mapped *MappedSpanError
	var noBars *NoBarsError
	var stored *StoreError
	if errors.As(err, &invalid) || errors.As(err, &duplicate) || errors.As(err, &mapped) ||
		errors.As(err, &noBars) || errors.As(err, &stored) {
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
		bars, err := fetch(start, spanEnd)
		if err != nil {
			return &fetchError{err: err}
		}
		return db.replaceAndScale(ctx, provider, symbol, first, start, spanEnd, bars, apply)
	}

	if end != nil && !end.After(last) {
		// The requested range is already stored.
		return nil
	}

	// A day missing inside the requested range (a provider gap on an earlier
	// fetch) would never be asked for again if the fetch began on the last
	// stored day, so it begins next to the first missing weekday instead.
	// Only the days from gapScanFrom on are looked at. Still one provider
	// call.
	scanFrom := gapScanFrom(start, last)
	storedDays, err := db.storedDays(ctx, provider, symbol, scanFrom, last)
	if err != nil {
		return err
	}
	plan := planFetch(scanFrom, last, storedDays)
	bars, err := fetch(plan.from, end)
	if err != nil {
		return &fetchError{err: err}
	}
	// Every check day is a stored day inside the fetched span. When one of
	// them came back on another scale nothing from this fetch is stored next
	// to the old rows: the whole history is fetched again and replaced.
	for _, day := range plan.checks {
		matches, err := db.overlapMatches(ctx, provider, symbol, day, bars)
		if err != nil {
			return err
		}
		if !matches {
			fresh, err := fetch(first, end)
			if err != nil {
				return &fetchError{err: err}
			}
			return db.replaceAndScale(ctx, provider, symbol, day, first, end, fresh, apply)
		}
	}
	return storeBars(newDaysOnly(bars, plan.from, last, storedDays))
}

// replaceAndScale replaces symbol and, in the same transaction, multiplies
// every other mapped ticker of that provider and instrument by the factor
// on day (fetched close over stored close). With no other ticker this is
// the replace Sync already did. A day with no comparable close fails when
// another ticker exists, and nothing is stored.
func (db *OHLC) replaceAndScale(ctx context.Context, provider, symbol string, day, from time.Time, to *time.Time,
	bars []*financepb.OHLC, apply bool,
) error {
	factor, older, err := db.oldTickerFactor(ctx, provider, symbol, day, from, to, bars)
	if err != nil {
		return err
	}
	if !apply {
		return nil
	}
	if !older {
		return db.ReplaceRange(ctx, provider, symbol, from, to, bars)
	}
	if len(bars) == 0 {
		return &NoBarsError{Provider: provider, Symbol: symbol, Start: from, End: to}
	}
	if err := ValidateBars(provider, symbol, bars); err != nil {
		return err
	}
	begin, ok := db.Conn.(txBeginner)
	if !ok {
		return ErrNoTransaction
	}
	tx, err := begin.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to replace ohlc: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if err := replaceStoredBars(ctx, tx, provider, symbol, from, to, bars); err != nil {
		return err
	}
	if err := scaleOldTickers(ctx, tx, provider, symbol, from, to, factor); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to replace ohlc: %w", err)
	}
	return nil
}

// MappedSpanError is one ticker mapped to more than one instrument inside
// the dates being written. Nothing is stored.
type MappedSpanError struct {
	Symbol   string
	Provider string
}

func (e *MappedSpanError) Error() string {
	return fmt.Sprintf("%s on %s is mapped for more than one instrument in the dates being written",
		e.Symbol, e.Provider)
}

func (db *OHLC) oldTickerFactor(ctx context.Context, provider, symbol string, day, from time.Time, to *time.Time, bars []*financepb.OHLC) (float64, bool, error) {
	older, err := db.hasOlderTicker(ctx, provider, symbol, from, to)
	if err != nil {
		return 0, false, err
	}
	if !older {
		return 0, false, nil
	}
	want := tradingDay(day)
	var fetched float64
	found := false
	for _, bar := range bars {
		if tradingDay(bar.GetTimestamp().AsTime()) != want {
			continue
		}
		fetched = bar.GetClose()
		found = true
		break
	}
	if !found || !positivePrice(fetched) {
		return 0, true, fmt.Errorf("can't re-scale %s: %s has no close on %s to compare with the stored one",
			symbol, priceProvider(provider), want)
	}
	stored, err := db.Close(ctx, provider, symbol, day)
	if err != nil {
		return 0, true, err
	}
	if !positivePrice(stored) {
		return 0, true, fmt.Errorf("can't re-scale %s: %s has no stored close on %s", symbol, priceProvider(provider), want)
	}
	factor := fetched / stored
	if !positivePrice(factor) {
		return 0, true, fmt.Errorf("can't re-scale %s: the factor on %s is not a price", symbol, want)
	}
	return factor, true, nil
}

// hasOlderTicker reports whether the mapping that covers the dates being
// written has an older ticker of the same instrument. mine is that one
// mapping: the provider, the symbol, and the written range. Another
// company's row for the same ticker is not mine, and a newer ticker of
// this instrument is not scaled.
func (db *OHLC) hasOlderTicker(ctx context.Context, provider, symbol string, from time.Time, to *time.Time) (bool, error) {
	mineID, mineFrom, ok, err := db.mineMapping(ctx, provider, symbol, from, to)
	if err != nil || !ok {
		return false, err
	}
	var older bool
	err = db.Conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM provider_symbol
			 WHERE instrument_id = $1 AND provider = $2
			   AND symbol <> $3 AND from_date < $4::date
		)`, mineID, priceProvider(provider), symbol, tradingDay(mineFrom)).Scan(&older)
	if err != nil {
		return false, fmt.Errorf("failed to read older tickers: %w", err)
	}
	return older, nil
}

func (db *OHLC) mineMapping(ctx context.Context, provider, symbol string, from time.Time, to *time.Time) (string, time.Time, bool, error) {
	provider = priceProvider(provider)
	spanEnd := "9999-12-31"
	if to != nil {
		spanEnd = tradingDay(*to)
	}
	rows, err := db.Conn.Query(ctx, `
		SELECT instrument_id, from_date
		  FROM provider_symbol
		 WHERE provider = $1 AND symbol = $2
		   AND from_date <= $4::date
		   AND (to_date IS NULL OR to_date > $3::date)`,
		provider, symbol, tradingDay(from), spanEnd)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("failed to read the mapping being written: %w", err)
	}
	defer rows.Close()
	var id string
	var fromDate time.Time
	n := 0
	for rows.Next() {
		n++
		if err := rows.Scan(&id, &fromDate); err != nil {
			return "", time.Time{}, false, fmt.Errorf("failed to read the mapping being written: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return "", time.Time{}, false, fmt.Errorf("failed to read the mapping being written: %w", err)
	}
	if n == 0 {
		return "", time.Time{}, false, nil
	}
	if n > 1 {
		return "", time.Time{}, false, &MappedSpanError{Symbol: symbol, Provider: provider}
	}
	return id, dateOnly(fromDate), true, nil
}

func scaleOldTickers(ctx context.Context, tx pgx.Tx, provider, symbol string, from time.Time, to *time.Time, factor float64) error {
	provider = priceProvider(provider)
	spanEnd := "9999-12-31"
	if to != nil {
		spanEnd = tradingDay(*to)
	}
	_, err := tx.Exec(ctx, `
		UPDATE ohlc o
		   SET open = open * $3, high = high * $3, low = low * $3, close = close * $3
		  FROM provider_symbol mine
		  JOIN provider_symbol other
		    ON other.instrument_id = mine.instrument_id
		   AND other.provider = mine.provider
		   AND other.symbol <> mine.symbol
		   AND other.from_date < mine.from_date
		 WHERE mine.provider = $1 AND mine.symbol = $2
		   AND mine.from_date <= $5::date
		   AND (mine.to_date IS NULL OR mine.to_date > $4::date)
		   AND o.provider = other.provider AND o.symbol = other.symbol
		   AND (o.time AT TIME ZONE 'UTC')::date >= other.from_date
		   AND (other.to_date IS NULL OR (o.time AT TIME ZONE 'UTC')::date < other.to_date)`,
		provider, symbol, factor, tradingDay(from), spanEnd)
	if err != nil {
		return fmt.Errorf("failed to re-scale older tickers of %s: %w", symbol, err)
	}
	return nil
}

// gapScanDays is how many calendar days before the last stored day an
// incremental Sync looks for missing weekdays. Ingest asks for the whole
// backtest range on every run, and an exchange holiday or a halted day is
// never filled by the provider, so scanning the whole stored range would make
// every daily sync fetch again from the first holiday in it (all of 2020
// stored: from 2020-01-20, hundreds of bars instead of two). Two weeks covers
// a provider gap from a recent fetch. Older gaps stay unfilled in storage,
// and backtest create fails on any of them that falls on a trading day;
// nothing fills them on read.
const gapScanDays = 14

// gapScanFrom is the first day an incremental Sync looks at for gaps: start,
// or gapScanDays before last when that is later. A gap is only filled when it
// is both inside the requested range and recent.
func gapScanFrom(start, last time.Time) time.Time {
	windowStart, _ := dayBounds(last.AddDate(0, 0, -gapScanDays))
	if start.Before(windowStart) {
		return windowStart
	}
	return start
}

// fetchPlan is the one span an incremental Sync fetches and the stored days
// whose fetched close must match before any of its bars is stored.
type fetchPlan struct {
	from   time.Time
	checks []time.Time
}

// planFetch picks the incremental fetch for a stored history whose last day
// is last, given the days stored from start (the gap scan start, see
// gapScanFrom) through last.
//
// With no missing weekday from start up to last, the fetch begins on last
// and last is the check day, as before.
//
// With a missing weekday, the fetch begins on the closest stored day before
// it, so that day and last are both checked. When no day before the gap is
// stored from start on (the gap is the first day scanned, which is also the
// case for a gap at the very start of the stored history), the fetch
// begins on the gap and the first stored day after it is checked instead,
// together with last. Either way the check days are inside the fetched span.
func planFetch(start, last time.Time, stored map[string]bool) fetchPlan {
	lastDay, _ := dayBounds(last)
	gap, ok := firstMissingWeekday(start, last, stored)
	if !ok {
		return fetchPlan{from: lastDay, checks: []time.Time{lastDay}}
	}
	if before, ok := storedDayBefore(start, gap, stored); ok {
		return fetchPlan{from: before, checks: []time.Time{before, lastDay}}
	}
	plan := fetchPlan{from: gap}
	if after, ok := storedDayAfter(gap, last, stored); ok && after.Before(lastDay) {
		plan.checks = append(plan.checks, after)
	}
	plan.checks = append(plan.checks, lastDay)
	return plan
}

// storedDays is the set of trading days stored for provider and symbol from
// the day of from through the day of to.
func (db *OHLC) storedDays(ctx context.Context, provider, symbol string, from, to time.Time) (map[string]bool, error) {
	spanStart, _ := dayBounds(from)
	_, spanEnd := dayBounds(to)
	rows, err := db.Conn.Query(ctx,
		`SELECT time FROM ohlc WHERE provider = $1 AND symbol = $2 AND time >= $3 AND time < $4`,
		priceProvider(provider), symbol, spanStart, spanEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to read stored ohlc days: %w", err)
	}
	defer rows.Close()
	days := map[string]bool{}
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("failed to read stored ohlc days: %w", err)
		}
		days[tradingDay(t)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read stored ohlc days: %w", err)
	}
	return days, nil
}

// firstMissingWeekday is the first Monday-to-Friday day from the day of from
// up to, but not including, the day of to that has no stored bar. Exchange
// holidays count as missing too: there is no holiday calendar here, and the
// provider simply returns no bar for them, so the cost is a longer span in the
// one fetch Sync makes anyway, never an extra call.
func firstMissingWeekday(from, to time.Time, stored map[string]bool) (time.Time, bool) {
	day, _ := dayBounds(from)
	stop, _ := dayBounds(to)
	for ; day.Before(stop); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		if !stored[tradingDay(day)] {
			return day, true
		}
	}
	return time.Time{}, false
}

// storedDayBefore is the latest stored day from the day of from up to, but
// not including, the day of gap.
func storedDayBefore(from, gap time.Time, stored map[string]bool) (time.Time, bool) {
	stop, _ := dayBounds(from)
	day, _ := dayBounds(gap)
	for day = day.AddDate(0, 0, -1); !day.Before(stop); day = day.AddDate(0, 0, -1) {
		if stored[tradingDay(day)] {
			return day, true
		}
	}
	return time.Time{}, false
}

// storedDayAfter is the first stored day after the day of gap, up to and
// including the day of to.
func storedDayAfter(gap, to time.Time, stored map[string]bool) (time.Time, bool) {
	day, _ := dayBounds(gap)
	stop, _ := dayBounds(to)
	for day = day.AddDate(0, 0, 1); !day.After(stop); day = day.AddDate(0, 0, 1) {
		if stored[tradingDay(day)] {
			return day, true
		}
	}
	return time.Time{}, false
}

// newDaysOnly keeps the fetched bars from the day of from on, except for
// days before last that are already stored, so a gap fill never rewrites a
// stored row there. The last stored day is kept: it matched within
// rescaleTolerance and is upserted as an incremental fetch always did.
func newDaysOnly(bars []*financepb.OHLC, from, last time.Time, stored map[string]bool) []*financepb.OHLC {
	fromDay, lastDay := tradingDay(from), tradingDay(last)
	kept := make([]*financepb.OHLC, 0, len(bars))
	for _, b := range bars {
		day := tradingDay(b.GetTimestamp().AsTime())
		if day < fromDay || (day < lastDay && stored[day]) {
			continue
		}
		kept = append(kept, b)
	}
	return kept
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

// dividendCache holds the dividends fetched before the lock: every ex-date
// from the day of from through today.
type dividendCache struct {
	fetch     FetchDividends
	from      time.Time
	dividends []supplier.Dividend
	fetchedAt time.Time
}

// prefetchDividends fetches, outside any transaction, the dividends of the
// span the stored bars will cover: from the earlier of start and the first
// stored day through today.
func (db *OHLC) prefetchDividends(ctx context.Context, provider, symbol string, start time.Time,
	fetch FetchDividends,
) (*dividendCache, error) {
	from := start
	first, _, stored, err := db.Range(ctx, provider, symbol)
	if err != nil {
		return nil, &StoreError{Symbol: symbol, Err: err}
	}
	if stored && first.Before(from) {
		from = first
	}
	cache := &dividendCache{fetch: fetch, from: from}
	if err := cache.load(from); err != nil {
		return nil, err
	}
	return cache, nil
}

func (c *dividendCache) load(from time.Time) error {
	fetchedAt := time.Now().UTC()
	dividends, err := c.fetch(from, nil)
	if err != nil {
		return err
	}
	c.from, c.dividends, c.fetchedAt = from, dividends, fetchedAt
	return nil
}

// replaceDividends stores the dividends of the span the bars cover now, in
// the Sync transaction. A span that starts before the prefetched one is
// fetched again here, under the lock. No stored bars means nothing to attach
// dividends to; the caller already failed or stored no bars on purpose.
func (db *OHLC) replaceDividends(ctx context.Context, provider, symbol string, cache *dividendCache) error {
	first, last, stored, err := db.Range(ctx, provider, symbol)
	if err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	if !stored {
		return nil
	}
	if tradingDay(first) < tradingDay(cache.from) {
		if err := cache.load(first); err != nil {
			return err
		}
	}
	dividends := Dividends{Conn: db.Conn}
	err = dividends.Replace(ctx, provider, symbol, first, last, cache.dividends, cache.fetchedAt)
	var invalid *DividendError
	if err != nil && !errors.As(err, &invalid) {
		return &StoreError{Symbol: symbol, Err: err}
	}
	return err
}

// SyncSource stores the bars of provider and symbol through the Sync rule,
// and for a provider whose bars are not dividend-adjusted (yahoo, alpaca)
// the dividends with them, in the same transaction. Stooq's bars already
// include dividends, so it stores bars only. A yahoo or alpaca source that
// can't fetch dividends is an error, not a run without them.
func (db *OHLC) SyncSource(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	fetch FetchBars, source supplier.Marketdata,
) error {
	return db.SyncSourceThen(ctx, provider, symbol, start, end, fetch, source, nil)
}

// SyncSourceThen is SyncSource, and then runs in the same transaction before
// commit. A refusal in either rolls the bars and then back.
func (db *OHLC) SyncSourceThen(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	fetch FetchBars, source supplier.Marketdata, then func(context.Context, pgx.Tx) error,
) error {
	switch priceProvider(provider) {
	case "stooq":
		return db.sync(ctx, provider, symbol, start, end, fetch, nil, then)
	case "yahoo", "alpaca":
		dividends, ok := source.(supplier.Dividends)
		if !ok {
			return &StoreError{Symbol: symbol, Err: fmt.Errorf("%s price source can't fetch dividends", provider)}
		}
		return db.sync(ctx, provider, symbol, start, end, fetch,
			func(from time.Time, to *time.Time) ([]supplier.Dividend, error) {
				found, err := dividends.GetDividends(ctx, symbol, from, to)
				if err != nil {
					return nil, DividendFetchFailure(provider, symbol, err)
				}
				return found, nil
			}, then)
	default:
		return &StoreError{Symbol: symbol, Err: fmt.Errorf("unknown price provider %q", provider)}
	}
}
