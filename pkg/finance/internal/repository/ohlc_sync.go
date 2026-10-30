package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

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

// StoreError is a database failure while storing one symbol's prices. Its
// text is a fixed sentence, so no SQL or driver detail reaches a task state
// or a client. Err keeps the cause for the server log.
type StoreError struct {
	Symbol string
	Err    error
}

func (e *StoreError) Error() string {
	return fmt.Sprintf("Couldn't store prices for %s. Try again in a few minutes. "+
		"If it keeps failing, report it at https://github.com/lhjnilsson/foreverbull/issues.", e.Symbol)
}

func (e *StoreError) Unwrap() error { return e.Err }

// FetchBars returns the provider's bars for one symbol from start through end
// (nil end: through today).
type FetchBars func(start time.Time, end *time.Time) ([]*financepb.OHLC, error)

// rescaleTolerance is how far the stored and fetched close of the overlap day
// may differ before the stored history is treated as being on another scale,
// for example after a split.
const rescaleTolerance = 0.005

// Sync brings the stored history of provider and symbol up to start through
// end. Ingest and the DownloadHistoricalData RPC both store bars through it,
// so both re-scale after a split the same way.
//
// Everything runs in one transaction whose first statement takes
// pg_advisory_xact_lock(hashtext('ohlc:' || symbol)). The stored range and the
// overlap close are read, and the append-or-replace decision is made, under
// that lock, so two syncs of one symbol run one after the other and can't
// leave days on two scales. fetch is called while the lock is held.
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
func (db *OHLC) Sync(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	fetch FetchBars,
) error {
	begin, ok := db.Conn.(txBeginner)
	if !ok {
		return &StoreError{Symbol: symbol, Err: ErrNoTransaction}
	}
	tx, err := begin.Begin(ctx)
	if err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || $1))`, symbol); err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	inTx := &OHLC{Conn: tx}
	if err := inTx.syncLocked(ctx, provider, symbol, start, end, fetch); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return &StoreError{Symbol: symbol, Err: err}
	}
	return nil
}

// fetchError marks an error that came from fetch, so Sync returns it as is.
type fetchError struct{ err error }

func (e *fetchError) Error() string { return e.err.Error() }
func (e *fetchError) Unwrap() error { return e.err }

func (db *OHLC) syncLocked(ctx context.Context, provider, symbol string,
	start time.Time, end *time.Time, fetch FetchBars,
) error {
	err := db.decide(ctx, provider, symbol, start, end, fetch)
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

func (db *OHLC) decide(ctx context.Context, provider, symbol string,
	start time.Time, end *time.Time, fetch FetchBars,
) error {
	first, last, stored, err := db.Range(ctx, provider, symbol)
	if err != nil {
		return err
	}

	if !stored {
		bars, err := fetch(start, end)
		if err != nil {
			return &fetchError{err: err}
		}
		return db.StoreBars(ctx, provider, symbol, bars)
	}

	if start.Before(first) {
		spanEnd := end
		if end != nil && end.Before(last) {
			spanEnd = &last
		}
		return db.refetchAndReplace(ctx, provider, symbol, start, spanEnd, fetch)
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
		return db.refetchAndReplace(ctx, provider, symbol, first, end, fetch)
	}
	return db.StoreBars(ctx, provider, symbol, bars)
}

// refetchAndReplace fetches start through end and makes it the stored history
// for that span. The batch is validated first, so a bad bar never turns into a
// missing day.
func (db *OHLC) refetchAndReplace(ctx context.Context, provider, symbol string,
	start time.Time, end *time.Time, fetch FetchBars,
) error {
	bars, err := fetch(start, end)
	if err != nil {
		return &fetchError{err: err}
	}
	return db.ReplaceRange(ctx, provider, symbol, start, end, bars)
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
