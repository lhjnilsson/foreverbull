package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const InstrumentTable = `
CREATE TABLE IF NOT EXISTS instrument (
	id text PRIMARY KEY,
	isin text UNIQUE,
	name text
);
`

const AssetSymbolTable = `
CREATE TABLE IF NOT EXISTS asset_symbol (
	isin text NOT NULL,
	symbol text NOT NULL,
	from_date date NOT NULL,
	to_date date,
	PRIMARY KEY (isin, from_date)
);
`

// AliasInterval is a half-open [FromDate, ToDate) ticker row in asset_symbol.
// Nil ToDate means the alias is current.
type AliasInterval struct {
	ISIN     string
	Symbol   string
	FromDate time.Time
	ToDate   *time.Time
}

// Instrument stores ISIN identity and dated ticker aliases for ETF universes.
type Instrument struct {
	Conn postgres.Query
}

func (db *Instrument) Upsert(ctx context.Context, isin, name string) error {
	isin = strings.TrimSpace(isin)
	if isin == "" {
		return fmt.Errorf("missing ISIN")
	}

	_, err := db.Conn.Exec(ctx,
		`INSERT INTO instrument(id, isin, name) VALUES($1, $1, $2)
		ON CONFLICT (id) DO UPDATE SET name = COALESCE(NULLIF(EXCLUDED.name, ''), instrument.name)`,
		isin, name)
	if err != nil {
		return fmt.Errorf("failed to upsert instrument: %w", err)
	}
	return nil
}

func (db *Instrument) StoreAlias(ctx context.Context, alias AliasInterval) error {
	isin := strings.TrimSpace(alias.ISIN)
	if isin == "" {
		return fmt.Errorf("missing ISIN")
	}
	symbol := strings.TrimSpace(alias.Symbol)
	if symbol == "" {
		return fmt.Errorf("missing symbol for isin %s", isin)
	}

	var to any
	if alias.ToDate != nil {
		to = dateString(*alias.ToDate)
	}

	_, err := db.Conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, $2, $3::date, $4::date)
		ON CONFLICT (isin, from_date) DO UPDATE
		SET symbol = EXCLUDED.symbol, to_date = EXCLUDED.to_date`,
		isin, symbol, dateString(alias.FromDate), to)
	if err != nil {
		return fmt.Errorf("failed to store asset_symbol: %w", err)
	}
	return nil
}

// SymbolAt returns the ticker of isin on date t ([from, to)).
func (db *Instrument) SymbolAt(ctx context.Context, isin string, t time.Time) (string, error) {
	isin = strings.TrimSpace(isin)
	if isin == "" {
		return "", fmt.Errorf("missing ISIN")
	}

	var symbol string
	err := db.Conn.QueryRow(ctx, `
		SELECT symbol FROM asset_symbol
		 WHERE isin = $1
		   AND from_date <= $2::date
		   AND (to_date IS NULL OR $2::date < to_date)`,
		isin, dateString(t)).Scan(&symbol)
	if err != nil {
		return "", fmt.Errorf("failed to get symbol at %s: %w", dateString(t), err)
	}
	return symbol, nil
}

// ListAliases returns ticker intervals for isin, ordered by from_date.
func (db *Instrument) ListAliases(ctx context.Context, isin string) ([]AliasInterval, error) {
	isin = strings.TrimSpace(isin)
	if isin == "" {
		return nil, fmt.Errorf("missing ISIN")
	}

	rows, err := db.Conn.Query(ctx, `
		SELECT isin, symbol, from_date, to_date
		  FROM asset_symbol
		 WHERE isin = $1
		 ORDER BY from_date`, isin)
	if err != nil {
		return nil, fmt.Errorf("failed to list asset_symbol: %w", err)
	}
	defer rows.Close()

	aliases := make([]AliasInterval, 0)
	for rows.Next() {
		var alias AliasInterval
		if err := rows.Scan(&alias.ISIN, &alias.Symbol, &alias.FromDate, &alias.ToDate); err != nil {
			return nil, fmt.Errorf("failed to scan asset_symbol: %w", err)
		}
		alias.FromDate = dateOnly(alias.FromDate)
		if alias.ToDate != nil {
			to := dateOnly(*alias.ToDate)
			alias.ToDate = &to
		}
		aliases = append(aliases, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list asset_symbol: %w", err)
	}
	return aliases, nil
}
