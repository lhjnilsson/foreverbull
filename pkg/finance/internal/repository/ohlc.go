package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
)

const OHLCTable = `CREATE TABLE IF NOT EXISTS ohlc (
symbol TEXT references asset(symbol),
open numeric,
high numeric,
low numeric,
close numeric,
volume bigint,
time timestamp,
provider text NOT NULL DEFAULT 'yahoo',
UNIQUE (provider, symbol, time));
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

func priceProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return "yahoo"
	}
	return provider
}

type OHLC struct {
	Conn postgres.Query
}

func (db *OHLC) Store(ctx context.Context, provider, symbol string, t time.Time, o, h, l, c float64, v int) error {
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
