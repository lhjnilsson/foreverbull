package repository

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
)

const OHLCTable = `CREATE TABLE IF NOT EXISTS ohlc (
symbol TEXT references asset(symbol),
open numeric NOT NULL,
high numeric NOT NULL,
low numeric NOT NULL,
close numeric NOT NULL,
volume bigint NOT NULL,
time timestamp,
provider text NOT NULL DEFAULT 'yahoo',
UNIQUE (provider, symbol, time),
CONSTRAINT ohlc_bar_check CHECK (high >= low AND volume >= 0),
CONSTRAINT ohlc_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca')));
`

// migrateOHLCProvider adds provider and replaces UNIQUE (symbol, time).
// Each statement is its own Exec. A DO block is one statement.
const migrateOHLCProviderColumn = `
ALTER TABLE ohlc ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'yahoo'`

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
// Rows stay. Existing rows become provider yahoo. Calling it twice is safe.
func MigrateOHLC(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, migrateOHLCProviderColumn); err != nil {
		return fmt.Errorf("ohlc provider column: %w", err)
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

func (db *OHLC) Store(ctx context.Context, provider, symbol string, t time.Time, o, h, l, c float64, v int) error {
	if c < 0 || math.IsNaN(c) || math.IsInf(c, 0) {
		return fmt.Errorf("close: %w", supplier.ErrRejected)
	}

	_, err := db.Conn.Exec(
		ctx,
		`INSERT into ohlc(provider, symbol, time, open, high, low, close, volume)
		values ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (provider, symbol, time) DO NOTHING`,
		priceProvider(provider), symbol, t, o, h, l, c, v)
	if err != nil {
		return fmt.Errorf("failed to store ohlc: %w", err)
	}

	return nil
}

// Close is the close stored for provider, symbol, and calendar day.
func (db *OHLC) Close(ctx context.Context, provider, symbol string, day time.Time) (float64, error) {
	var closePx float64
	err := db.Conn.QueryRow(
		ctx,
		`SELECT close::float8 FROM ohlc
		 WHERE provider = $1 AND symbol = $2 AND time::date = $3::date`,
		priceProvider(provider), symbol, day.UTC().Format("2006-01-02"),
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
	for _, symbol := range symbols {
		err := db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE provider = $1 AND symbol = $2 AND time::date = $3)",
			provider, symbol, pb.DateToDateString(start)).Scan(&startExists)
		if err != nil {
			return false, fmt.Errorf("failed to check if ohlc exists: %w", err)
		}

		err = db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE provider = $1 AND symbol = $2 AND time::date = $3)",
			provider, symbol, pb.DateToDateString(end)).Scan(&endExists)
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
