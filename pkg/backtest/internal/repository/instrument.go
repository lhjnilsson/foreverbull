package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// InstrumentTable and AssetSymbolTable match pkg/finance/internal/repository/instrument.go.
const InstrumentTable = `
CREATE TABLE IF NOT EXISTS instrument (
	id text PRIMARY KEY,
	isin text UNIQUE,
	name text
);
`

const assetSymbolSymbolIndex = `CREATE INDEX IF NOT EXISTS asset_symbol_symbol_from_date_idx ON asset_symbol (symbol, from_date);
`

const AssetSymbolTable = `
CREATE TABLE IF NOT EXISTS asset_symbol (
	isin text NOT NULL,
	symbol text NOT NULL,
	from_date date NOT NULL,
	to_date date,
	PRIMARY KEY (isin, from_date),
	CONSTRAINT asset_symbol_isin_fkey FOREIGN KEY (isin) REFERENCES instrument (isin),
	CONSTRAINT asset_symbol_no_overlap EXCLUDE USING gist (
		isin WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	)
);

` + assetSymbolSymbolIndex

const addAssetSymbolForeignKey = `
ALTER TABLE asset_symbol
	ADD CONSTRAINT asset_symbol_isin_fkey
	FOREIGN KEY (isin) REFERENCES instrument (isin)`

const addAssetSymbolExclusion = `
ALTER TABLE asset_symbol
	ADD CONSTRAINT asset_symbol_no_overlap
	EXCLUDE USING gist (
		isin WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	)`

// MigrateAssetSymbol adds the instrument foreign key, the per-ISIN
// half-open exclusion, and the (symbol, from_date) index.
// Rows that would violate the new constraint stay. Calling it twice is safe.
func MigrateAssetSymbol(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("failed to create extension btree_gist: %w", err)
	}
	if err := migrateAssetSymbolForeignKey(ctx, conn); err != nil {
		return err
	}
	if err := migrateAssetSymbolExclusion(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, assetSymbolSymbolIndex); err != nil {
		return fmt.Errorf("asset_symbol: symbol index: %w", err)
	}
	return nil
}

func migrateAssetSymbolForeignKey(ctx context.Context, conn postgres.Query) error {
	exists, err := tableConstraintExists(ctx, conn, "asset_symbol", "asset_symbol_isin_fkey")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var bad bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM asset_symbol s
			LEFT JOIN instrument i ON i.isin = s.isin
			WHERE i.isin IS NULL
		)`).Scan(&bad)
	if err != nil {
		return fmt.Errorf("asset_symbol: scan foreign key: %w", err)
	}
	if bad {
		return fmt.Errorf("asset_symbol: rows violate foreign key isin references instrument(isin)")
	}

	if _, err = conn.Exec(ctx, addAssetSymbolForeignKey); err != nil {
		return fmt.Errorf("asset_symbol: foreign key: %w", err)
	}
	return nil
}

func migrateAssetSymbolExclusion(ctx context.Context, conn postgres.Query) error {
	exists, err := tableConstraintExists(ctx, conn, "asset_symbol", "asset_symbol_no_overlap")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var bad bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM asset_symbol a
			JOIN asset_symbol b
			  ON a.isin = b.isin
			 AND a.from_date < b.from_date
			 AND daterange(a.from_date, a.to_date, '[)') && daterange(b.from_date, b.to_date, '[)')
		)`).Scan(&bad)
	if err != nil {
		return fmt.Errorf("asset_symbol: scan overlap: %w", err)
	}
	if bad {
		return fmt.Errorf("asset_symbol: rows violate exclusion on isin and daterange(from_date, to_date, '[)')")
	}

	if _, err = conn.Exec(ctx, addAssetSymbolExclusion); err != nil {
		return fmt.Errorf("asset_symbol: exclusion: %w", err)
	}
	return nil
}

func tableConstraintExists(ctx context.Context, conn postgres.Query, table, name string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint c
			JOIN pg_class t ON t.oid = c.conrelid
			JOIN pg_namespace n ON n.oid = t.relnamespace
			WHERE n.nspname = current_schema()
			  AND t.relname = $1
			  AND c.conname = $2
		)`, table, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: read constraint %s: %w", table, name, err)
	}
	return exists, nil
}

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
