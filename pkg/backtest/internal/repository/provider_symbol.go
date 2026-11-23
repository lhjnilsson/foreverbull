package repository

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// ProviderSymbolTable matches pkg/finance/internal/repository/provider_symbol.go.
const ProviderSymbolTable = `
CREATE TABLE IF NOT EXISTS provider_symbol (
	instrument_id text NOT NULL REFERENCES instrument (id),
	provider text NOT NULL,
	symbol text NOT NULL,
	from_date date NOT NULL,
	to_date date,
	evidence text NOT NULL,
	PRIMARY KEY (instrument_id, provider, from_date),
	CONSTRAINT provider_symbol_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca')),
	CONSTRAINT provider_symbol_evidence_check CHECK (evidence IN ('download', 'snapshot', 'migration')),
	CONSTRAINT provider_symbol_no_overlap EXCLUDE USING gist (
		instrument_id WITH =,
		provider WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	),
	CONSTRAINT provider_symbol_ticker_no_overlap EXCLUDE USING gist (
		provider WITH =,
		symbol WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	)
);
`

const instrumentOHLCView = `
CREATE OR REPLACE VIEW instrument_ohlc AS
SELECT p.instrument_id, o.provider, o.time, o.open, o.high, o.low, o.close, o.volume, o.symbol
  FROM provider_symbol p
  JOIN ohlc o ON o.provider = p.provider AND o.symbol = p.symbol
 WHERE (o.time AT TIME ZONE 'UTC')::date >= p.from_date
   AND (p.to_date IS NULL OR (o.time AT TIME ZONE 'UTC')::date < p.to_date);
`

// MigrateProviderSymbol creates provider_symbol and, when ohlc exists,
// instrument_ohlc. The finance startup backfill fills the rows.
func MigrateProviderSymbol(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("failed to create extension btree_gist: %w", err)
	}
	if _, err := conn.Exec(ctx, ProviderSymbolTable); err != nil {
		return fmt.Errorf("provider_symbol: %w", err)
	}
	var ohlc bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('ohlc') IS NOT NULL`).Scan(&ohlc); err != nil {
		return fmt.Errorf("instrument_ohlc: %w", err)
	}
	if !ohlc {
		return nil
	}
	if _, err := conn.Exec(ctx, instrumentOHLCView); err != nil {
		return fmt.Errorf("instrument_ohlc: %w", err)
	}
	return nil
}
