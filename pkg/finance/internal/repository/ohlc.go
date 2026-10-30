package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const OHLCTable = `CREATE TABLE IF NOT EXISTS ohlc (
symbol TEXT references asset(symbol),
open numeric NOT NULL,
high numeric NOT NULL,
low numeric NOT NULL,
close numeric NOT NULL,
volume bigint NOT NULL,
time timestamp,
provider text NOT NULL,
UNIQUE (provider, symbol, time),
CONSTRAINT ohlc_bar_check CHECK (high >= low AND volume >= 0),
CONSTRAINT ohlc_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca')));
`

// migrateOHLCProvider adds provider and replaces UNIQUE (symbol, time).
// Each statement is its own Exec. A DO block is one statement.
const migrateOHLCProviderColumn = `
ALTER TABLE ohlc ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'yahoo'`

// migrateOHLCProviderDefault drops the 'yahoo' default once existing rows are
// filled, so an insert that forgets the provider fails instead of storing
// stooq or alpaca bars as yahoo.
const migrateOHLCProviderDefault = `
ALTER TABLE ohlc ALTER COLUMN provider DROP DEFAULT`

const migrateOHLCProviderKey = `
DO $$
DECLARE
	r record;
BEGIN
	FOR r IN
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = 'ohlc'
		  AND n.nspname = current_schema()
		  AND c.contype = 'u'
		  AND (
			c.conname IN ('ohlc_symbol_time_key', 'symbol_time_uc')
			OR regexp_replace(pg_get_constraintdef(c.oid), '\s+', ' ', 'g') IN (
				'UNIQUE (symbol, time)',
				'UNIQUE (symbol, "time")'
			)
		  )
	LOOP
		EXECUTE format('ALTER TABLE ohlc DROP CONSTRAINT %I', r.conname);
	END LOOP;

	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = 'ohlc'
		  AND n.nspname = current_schema()
		  AND c.conname = 'ohlc_provider_symbol_time_key'
	) THEN
		ALTER TABLE ohlc
			ADD CONSTRAINT ohlc_provider_symbol_time_key UNIQUE (provider, symbol, time);
	END IF;
END $$`

// MigrateOHLC adds provider to an existing ohlc table and replaces
// UNIQUE (symbol, time) with UNIQUE (provider, symbol, time).
// Rows stay. Existing rows become provider yahoo. New rows must name their
// provider. Calling it twice is safe.
func MigrateOHLC(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, migrateOHLCProviderColumn); err != nil {
		return fmt.Errorf("ohlc provider column: %w", err)
	}
	if _, err := conn.Exec(ctx, migrateOHLCProviderDefault); err != nil {
		return fmt.Errorf("ohlc provider default: %w", err)
	}
	if _, err := conn.Exec(ctx, migrateOHLCProviderKey); err != nil {
		return fmt.Errorf("ohlc provider key: %w", err)
	}
	return nil
}

// MigrateOHLCBars sets open, high, low, close, and volume NOT NULL and adds
// high >= low, volume >= 0, and the yahoo/stooq/alpaca provider check.
// Call it after MigrateOHLC. Violating rows stay. Calling it twice is safe.
func MigrateOHLCBars(ctx context.Context, conn postgres.Query) error {
	var bad bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM ohlc
			WHERE open IS NULL
			   OR high IS NULL
			   OR low IS NULL
			   OR close IS NULL
			   OR volume IS NULL
			   OR high < low
			   OR volume < 0
			   OR provider IS NULL
			   OR provider NOT IN ('yahoo', 'stooq', 'alpaca')
		)`).Scan(&bad)
	if err != nil {
		return fmt.Errorf("ohlc: scan bars: %w", err)
	}
	if bad {
		return fmt.Errorf("ohlc: rows violate NOT NULL, high >= low, volume >= 0, or provider")
	}

	statements := []string{
		`ALTER TABLE ohlc ALTER COLUMN open SET NOT NULL`,
		`ALTER TABLE ohlc ALTER COLUMN high SET NOT NULL`,
		`ALTER TABLE ohlc ALTER COLUMN low SET NOT NULL`,
		`ALTER TABLE ohlc ALTER COLUMN close SET NOT NULL`,
		`ALTER TABLE ohlc ALTER COLUMN volume SET NOT NULL`,
	}
	for _, statement := range statements {
		if _, err = conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("ohlc: not null: %w", err)
		}
	}

	if err = addOHLCCheck(ctx, conn, "ohlc_bar_check", `
		ALTER TABLE ohlc
			ADD CONSTRAINT ohlc_bar_check
			CHECK (high >= low AND volume >= 0)`); err != nil {
		return err
	}
	return addOHLCCheck(ctx, conn, "ohlc_provider_check", `
		ALTER TABLE ohlc
			ADD CONSTRAINT ohlc_provider_check
			CHECK (provider IN ('yahoo', 'stooq', 'alpaca'))`)
}

func addOHLCCheck(ctx context.Context, conn postgres.Query, name, statement string) error {
	exists, err := tableConstraintExists(ctx, conn, "ohlc", name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err = conn.Exec(ctx, statement); err != nil {
		return fmt.Errorf("ohlc: %s: %w", name, err)
	}
	return nil
}

// priceProvider resolves a blank provider to supplier.DefaultProvider for both
// writes and reads, so a bar stored without a name is found by a read without
// one, and no write falls through to the column's legacy 'yahoo' default.
func priceProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return supplier.DefaultProvider
	}
	return provider
}

type OHLC struct {
	Conn postgres.Query
}

// ErrInvalidPrice is a bar Store refuses because a price is 0, negative, or
// not a real number, or because its volume doesn't fit ohlc.volume. The error
// text names the provider, symbol and day.
var ErrInvalidPrice = errors.New("invalid price")

// InvalidPriceError is the user-facing refusal for one bar. The provider sent
// it, so the sentence says so rather than blaming the user. A refused volume
// has its own sentence; both match ErrInvalidPrice.
type InvalidPriceError struct {
	Provider string
	Symbol   string
	Day      time.Time
	// Field is the first refused column: open, high, low, close, or volume.
	Field string
}

func (e *InvalidPriceError) Error() string {
	day := e.Day.UTC().Format("2006-01-02")
	if e.Field == "volume" {
		return fmt.Sprintf("%s sent an invalid volume for %s on %s (out of range)", e.Provider, e.Symbol, day)
	}
	return fmt.Sprintf("%s sent an invalid price for %s on %s (0, negative or not a number)",
		e.Provider, e.Symbol, day)
}

func (e *InvalidPriceError) Unwrap() error { return ErrInvalidPrice }

// Store upserts one bar. Every price must be a real positive number: a 0
// price is how a missing provider bar used to arrive, and it reads as a
// -100% day, so it is refused here as well as in the provider adapter. A
// re-fetched bar replaces the stored one, so prices adjusted after a split
// never sit next to rows on the old scale. The volume must fit ohlc.volume.
func (db *OHLC) Store(ctx context.Context, provider, symbol string, t time.Time, o, h, l, c float64, v uint64) error {
	if err := validateBar(provider, symbol, t, o, h, l, c, v); err != nil {
		return err
	}
	// validateBar refused this already; the bound stays next to the
	// conversion so it is visibly in range.
	if v > maxVolume {
		return &InvalidPriceError{Provider: priceProvider(provider), Symbol: symbol, Day: t, Field: "volume"}
	}
	volume := int64(v)
	dayStart, dayEnd := dayBounds(t)

	// One row per trading day: a bar for a day already stored at another time
	// of day (00:00Z before, 05:00Z now) replaces that row instead of sitting
	// next to it on another scale. One statement, so it is atomic on its own.
	_, err := db.Conn.Exec(
		ctx,
		`WITH same_day AS (
			DELETE FROM ohlc
			WHERE provider = $1 AND symbol = $2
			  AND time >= $9 AND time < $10 AND time <> $3::timestamp
		)
		INSERT into ohlc(provider, symbol, time, open, high, low, close, volume)
		values ($1, $2, $3::timestamp, $4, $5, $6, $7, $8)
		ON CONFLICT (provider, symbol, time) DO UPDATE SET
			open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
			close = EXCLUDED.close, volume = EXCLUDED.volume`,
		priceProvider(provider), symbol, t, o, h, l, c, volume, dayStart, dayEnd)
	if err != nil {
		return fmt.Errorf("failed to store ohlc: %w", err)
	}

	return nil
}

// txBeginner is a connection that can open a transaction: a pool, a
// connection, or a transaction (which opens a savepoint).
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ErrNoTransaction is returned by StoreBars and ReplaceRange when their
// connection can't open a transaction. Bars are never stored row by row outside one, because a DB
// error part way through would leave a gap in the cached history.
var ErrNoTransaction = errors.New("ohlc: connection can't open a transaction")

// StoreBars validates and stores one symbol's batch in a single transaction,
// so a DB error on any row leaves none of the batch behind. A connection that
// can't open a transaction is an error; nothing is stored. The transaction
// covers ohlc rows only: callers write the asset and asset_symbol rows
// before it, and those stay when the bars roll back.
func (db *OHLC) StoreBars(ctx context.Context, provider, symbol string, bars []*financepb.OHLC) error {
	if err := ValidateBars(provider, symbol, bars); err != nil {
		return err
	}
	conn, ok := db.Conn.(txBeginner)
	if !ok {
		return ErrNoTransaction
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		inTx := OHLC{Conn: tx}
		return inTx.storeEach(ctx, provider, symbol, bars)
	})
}

func (db *OHLC) storeEach(ctx context.Context, provider, symbol string, bars []*financepb.OHLC) error {
	for _, bar := range bars {
		if err := db.Store(ctx, provider, symbol, bar.GetTimestamp().AsTime(),
			bar.GetOpen(), bar.GetHigh(), bar.GetLow(), bar.GetClose(), bar.GetVolume()); err != nil {
			return err
		}
	}
	return nil
}

// ValidateBars checks every bar a provider returned before any is stored.
// Callers run it first so a refused bar leaves no partial history behind:
// bars are cached per provider and symbol, and a gap there would be served to
// every later backtest.
func ValidateBars(provider, symbol string, bars []*financepb.OHLC) error {
	for _, bar := range bars {
		err := validateBar(provider, symbol, bar.GetTimestamp().AsTime(),
			bar.GetOpen(), bar.GetHigh(), bar.GetLow(), bar.GetClose(), bar.GetVolume())
		if err != nil {
			return err
		}
	}
	return nil
}

// maxVolume is the largest volume ohlc.volume (bigint) holds.
const maxVolume = math.MaxInt64

func validateBar(provider, symbol string, t time.Time, o, h, l, c float64, volume uint64) error {
	for _, price := range []struct {
		field string
		value float64
	}{{"open", o}, {"high", h}, {"low", l}, {"close", c}} {
		if !positivePrice(price.value) {
			return &InvalidPriceError{Provider: priceProvider(provider), Symbol: symbol, Day: t, Field: price.field}
		}
	}
	if volume > maxVolume {
		return &InvalidPriceError{Provider: priceProvider(provider), Symbol: symbol, Day: t, Field: "volume"}
	}
	return nil
}

func positivePrice(price float64) bool {
	return price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

// Range returns the first and last stored bar times for provider and symbol.
// ok is false when nothing is stored.
func (db *OHLC) Range(ctx context.Context, provider, symbol string) (first, last time.Time, ok bool, err error) {
	var minTime, maxTime *time.Time
	err = db.Conn.QueryRow(
		ctx,
		`SELECT MIN(time), MAX(time) FROM ohlc WHERE provider = $1 AND symbol = $2`,
		priceProvider(provider), symbol,
	).Scan(&minTime, &maxTime)
	if err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("failed to read ohlc range: %w", err)
	}
	if minTime == nil || maxTime == nil {
		return time.Time{}, time.Time{}, false, nil
	}
	return *minTime, *maxTime, true, nil
}

// ReplaceRange makes bars the whole stored history of provider and symbol
// for the requested trading days start through end (nil end: every stored day
// from start on), in one transaction. The span comes from the request, not
// from the bars, so a stored day after the provider's last bar is removed too
// and can't stay on an old scale. Days are compared by trading date, the same
// key Store upserts on, so a row stored at another time of day is replaced.
//
// A refetch that returns no bars is a NoBarsError, never a silent success.
// Every bar must pass ValidateBars; ReplaceRange checks the batch before it
// opens the transaction, so a refused bar leaves every stored row as it was.
// A connection that can't open a transaction returns ErrNoTransaction and
// nothing is written or removed.
func (db *OHLC) ReplaceRange(ctx context.Context, provider, symbol string, start time.Time, end *time.Time,
	bars []*financepb.OHLC,
) error {
	if len(bars) == 0 {
		return &NoBarsError{Provider: provider, Symbol: symbol, Start: start, End: end}
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

	spanStart, _ := dayBounds(start)
	var spanEnd *time.Time
	if end != nil {
		_, next := dayBounds(*end)
		spanEnd = &next
	}
	_, err = tx.Exec(ctx,
		`DELETE FROM ohlc WHERE provider = $1 AND symbol = $2
		 AND time >= $3 AND ($4::timestamp IS NULL OR time < $4::timestamp)`,
		priceProvider(provider), symbol, spanStart, spanEnd)
	if err != nil {
		return fmt.Errorf("failed to replace ohlc: %w", err)
	}
	inTx := &OHLC{Conn: tx}
	for _, b := range bars {
		err := inTx.Store(ctx, provider, symbol, b.GetTimestamp().AsTime(),
			b.GetOpen(), b.GetHigh(), b.GetLow(), b.GetClose(), b.GetVolume())
		if err != nil {
			return fmt.Errorf("failed to replace ohlc: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to replace ohlc: %w", err)
	}
	return nil
}

// tradingDay is the UTC calendar day of t, the key ohlc rows are compared on.
func tradingDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// dayBounds is the trading day of t as [start, next): 00:00 UTC of t's UTC
// day and 00:00 UTC of the day after. SQL compares time >= start AND
// time < next instead of time::date, so the comparison doesn't depend on the
// session TimeZone (should ohlc.time ever be timestamptz) and can use the
// (provider, symbol, time) index.
func dayBounds(t time.Time) (time.Time, time.Time) {
	u := t.UTC()
	start := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 0, 1)
}

// dateBounds is dayBounds for a calendar date.
func dateBounds(d *pb.Date) (time.Time, time.Time) {
	return dayBounds(time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC))
}

// Close is the close stored for provider, symbol, and calendar day.
func (db *OHLC) Close(ctx context.Context, provider, symbol string, day time.Time) (float64, error) {
	var closePx float64
	dayStart, dayEnd := dayBounds(day)
	err := db.Conn.QueryRow(
		ctx,
		`SELECT close::float8 FROM ohlc
		 WHERE provider = $1 AND symbol = $2 AND time >= $3 AND time < $4`,
		priceProvider(provider), symbol, dayStart, dayEnd,
	).Scan(&closePx)
	if err != nil {
		return 0, fmt.Errorf("failed to read ohlc close: %w", err)
	}
	return closePx, nil
}

func (db *OHLC) Exists(ctx context.Context, provider string, symbols []string, start, end *pb.Date) (bool, error) {
	var startExists bool

	var endExists bool

	provider = priceProvider(provider)
	startFrom, startTo := dateBounds(start)
	endFrom, endTo := dateBounds(end)
	for _, symbol := range symbols {
		err := db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE provider = $1 AND symbol = $2 AND time >= $3 AND time < $4)",
			provider, symbol, startFrom, startTo).Scan(&startExists)
		if err != nil {
			return false, fmt.Errorf("failed to check if ohlc exists: %w", err)
		}

		err = db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE provider = $1 AND symbol = $2 AND time >= $3 AND time < $4)",
			provider, symbol, endFrom, endTo).Scan(&endExists)
		if err != nil {
			return false, fmt.Errorf("failed to check if ohlc exists: %w", err)
		}

		if !startExists || !endExists {
			return false, nil
		}
	}

	return true, nil
}

func (db *OHLC) MinMax(ctx context.Context) (*pb.Date, *pb.Date, error) {
	var minTime *time.Time

	var maxTime *time.Time

	err := db.Conn.QueryRow(
		ctx,
		"SELECT MIN(time), MAX(time) FROM ohlc").Scan(&minTime, &maxTime)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get min/max time: %w", err)
	}

	if minTime == nil || maxTime == nil {
		return nil, nil, nil
	}

	return pb.GoTimeToDate(*minTime), pb.GoTimeToDate(*maxTime), nil
}

// maxOHLCListSymbols and maxOHLCListLimit bound GetOHLC / List input.
const (
	maxOHLCListSymbols   = 500
	maxOHLCListLimit     = 2520
	defaultOHLCListLimit = 252
)

// storedSymbols maps each asked symbol to the name its bars are stored
// under (BRK.B and BRK-B read BRKB), once each. Bars come back under the
// stored name.
func storedSymbols(symbols []string) []string {
	out := make([]string, 0, len(symbols))
	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		name := marketdata.StoredSymbol(symbol)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// List returns stored bars for provider and symbols with time <= asOf,
// at most limit bars per symbol (newest first in the window, returned oldest
// first). limit 0 means 252. Symbols with no bars are omitted. A listed
// share class is read under its stored name (BRK.B reads BRKB).
func (db *OHLC) List(ctx context.Context, provider string, symbols []string, asOf time.Time, limit int) ([]*financepb.OHLC, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("symbols: empty")
	}
	if len(symbols) > maxOHLCListSymbols {
		return nil, fmt.Errorf("symbols: at most %d", maxOHLCListSymbols)
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit: negative")
	}
	if limit == 0 {
		limit = defaultOHLCListLimit
	}
	if limit > maxOHLCListLimit {
		return nil, fmt.Errorf("limit: at most %d", maxOHLCListLimit)
	}
	symbols = storedSymbols(symbols)
	provider = priceProvider(provider)

	rows, err := db.Conn.Query(ctx, `
		SELECT symbol, time, open::float8, high::float8, low::float8, close::float8, volume
		FROM (
			SELECT symbol, time, open, high, low, close, volume,
			       ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY time DESC, ctid DESC) AS rn
			FROM ohlc
			WHERE provider = $1 AND symbol = ANY($2) AND time <= $3
		) bounded
		WHERE rn <= $4
		ORDER BY symbol ASC, time ASC`,
		provider, symbols, asOf.UTC(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list ohlc: %w", err)
	}
	defer rows.Close()

	out := make([]*financepb.OHLC, 0)
	for rows.Next() {
		var (
			symbol     string
			t          time.Time
			o, h, l, c float64
			volume     int64
		)
		if err := rows.Scan(&symbol, &t, &o, &h, &l, &c, &volume); err != nil {
			return nil, fmt.Errorf("failed to scan ohlc: %w", err)
		}
		if volume < 0 {
			return nil, fmt.Errorf("ohlc volume negative for %s", symbol)
		}
		out = append(out, &financepb.OHLC{
			Symbol:    symbol,
			Timestamp: timestamppb.New(t.UTC()),
			Open:      o,
			High:      h,
			Low:       l,
			Close:     c,
			Volume:    uint64(volume),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate ohlc: %w", err)
	}
	return out, nil
}

// stockDataWindow matches the client's exclusive lookback: 252 trading days
// span about a year, and twice that plus a month covers holidays and halts.
const stockDataWindow = time.Duration(defaultOHLCListLimit*2+30) * 24 * time.Hour

// ListResolved returns stored bars for the tickers the algorithm asked for.
// A renamed ticker (FB then META, one ISIN) is labeled with the asked symbol.
// time is inside (asOf-window, asOf], at most limit bars per symbol. limit 0
// means 252. An empty symbol list returns no bars. A listed share class is
// read under its stored name (BRK.B reads BRKB). Errors do not include a
// database URL.
func (db *OHLC) ListResolved(ctx context.Context, provider string, symbols []string, asOf time.Time, limit int) ([]*financepb.OHLC, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit: negative")
	}
	if limit == 0 {
		limit = defaultOHLCListLimit
	}
	if limit > maxOHLCListLimit {
		return nil, fmt.Errorf("limit: at most %d", maxOHLCListLimit)
	}
	symbols = storedSymbols(symbols)
	provider = priceProvider(provider)
	asOf = asOf.UTC()
	since := asOf.Add(-stockDataWindow)

	rows, err := db.Conn.Query(ctx, `
		WITH owner AS (
			SELECT DISTINCT ON (a.symbol) a.symbol AS wanted, a.isin
			FROM asset_symbol a
			WHERE a.symbol = ANY($1::text[])
			ORDER BY a.symbol,
			         (a.from_date <= $5::date AND (a.to_date IS NULL OR a.to_date > $5::date)) DESC,
			         (a.from_date <= $5::date) DESC,
			         a.from_date DESC
		),
		candidate AS (
			SELECT o.symbol AS wanted, o.symbol AS ticker, NULL::date AS from_date, NULL::date AS to_date,
			       o.time, o.high, o.low, o.open, o.close, o.volume
			FROM ohlc o
			WHERE o.symbol = ANY($1::text[]) AND o.provider = $2 AND o.time > $3 AND o.time <= $4
			UNION ALL
			SELECT w.wanted, o.symbol, s.from_date, s.to_date, o.time, o.high, o.low, o.open, o.close, o.volume
			FROM owner w
			JOIN asset_symbol s ON s.isin = w.isin
			JOIN ohlc o ON o.symbol = s.symbol
			WHERE o.provider = $2 AND o.time > $3 AND o.time <= $4
		),
		chosen AS (
			SELECT DISTINCT ON (wanted, time) wanted AS symbol, time, high, low, open, close, volume
			FROM candidate
			ORDER BY wanted, time,
			         CASE
			             WHEN from_date <= time::date AND (to_date IS NULL OR time::date < to_date) THEN 0
			             WHEN ticker = wanted THEN 1
			             ELSE 2
			         END
		)
		SELECT symbol, time, open::float8, high::float8, low::float8, close::float8, volume
		FROM (
			SELECT symbol, time, open, high, low, close, volume,
			       ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY time DESC) AS rn
			FROM chosen
		) bounded
		WHERE rn <= $6
		ORDER BY symbol ASC, time ASC`,
		symbols, provider, since, asOf, asOf, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list resolved ohlc: %w", err)
	}
	defer rows.Close()

	out := make([]*financepb.OHLC, 0)
	for rows.Next() {
		var (
			symbol     string
			t          time.Time
			o, h, l, c float64
			volume     int64
		)
		if err := rows.Scan(&symbol, &t, &o, &h, &l, &c, &volume); err != nil {
			return nil, fmt.Errorf("failed to scan ohlc: %w", err)
		}
		if volume < 0 {
			return nil, fmt.Errorf("ohlc volume negative for %s", symbol)
		}
		out = append(out, &financepb.OHLC{
			Symbol:    symbol,
			Timestamp: timestamppb.New(t.UTC()),
			Open:      o,
			High:      h,
			Low:       l,
			Close:     c,
			Volume:    uint64(volume),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate ohlc: %w", err)
	}
	return out, nil
}
