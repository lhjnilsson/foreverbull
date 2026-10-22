package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

// ErrAliasConflict is two different tickers for one ISIN on the same day.
// It carries the name of the table constraint that refuses the same rows.
var ErrAliasConflict = errors.New("asset_symbol_no_overlap")

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// StoreAlias records one ticker interval of an ETF backtest in asset_symbol,
// which every ETF backtest shares (universe.md §4.3).
//
// A NULL to_date only says the ticker was still current in the last list that
// backtest read. So an open-ended row yields to a different ticker that
// starts after it: a stored one is closed where the new ticker starts, and a
// new one is capped at the next stored row of another ticker. Rows of the
// same ticker that overlap are one alias and are stored as their union, where
// a known end wins over an open one. Any other overlap of two tickers,
// including the same from_date, is refused with ErrAliasConflict.
func (db *Instrument) StoreAlias(ctx context.Context, alias AliasInterval) error {
	isin := strings.TrimSpace(alias.ISIN)
	if isin == "" {
		return fmt.Errorf("missing ISIN")
	}
	symbol := strings.TrimSpace(alias.Symbol)
	if symbol == "" {
		return fmt.Errorf("missing symbol for isin %s", isin)
	}
	in := AliasInterval{ISIN: isin, Symbol: symbol, FromDate: dateOnly(alias.FromDate)}
	if alias.ToDate != nil {
		to := dateOnly(*alias.ToDate)
		if !to.After(in.FromDate) {
			return fmt.Errorf("symbol %s for isin %s ends %s, not after it starts %s",
				symbol, isin, dateString(to), dateString(in.FromDate))
		}
		in.ToDate = &to
	}

	begin, ok := db.Conn.(txBeginner)
	if !ok {
		return fmt.Errorf("failed to store asset_symbol: connection cannot begin a transaction")
	}
	tx, err := begin.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to store asset_symbol: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	// Creates run one at a time (universe.md §4.3), so the row lock is enough;
	// the exclusion constraint still refuses whatever a concurrent writer adds.
	stored, err := listAliases(ctx, tx, isin, " FOR UPDATE")
	if err != nil {
		return err
	}
	plan, err := planAlias(stored, in)
	if err != nil {
		return err
	}
	for _, from := range plan.remove {
		if _, err := tx.Exec(ctx, `DELETE FROM asset_symbol WHERE isin = $1 AND from_date = $2::date`,
			isin, dateString(from)); err != nil {
			return fmt.Errorf("failed to store asset_symbol: %w", err)
		}
	}
	for _, closed := range plan.close {
		if _, err := tx.Exec(ctx, `UPDATE asset_symbol SET to_date = $3::date WHERE isin = $1 AND from_date = $2::date`,
			isin, dateString(closed.FromDate), dateString(*closed.ToDate)); err != nil {
			return fmt.Errorf("failed to close asset_symbol: %w", err)
		}
	}
	var to any
	if plan.row.ToDate != nil {
		to = dateString(*plan.row.ToDate)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES ($1, $2, $3::date, $4::date)`,
		isin, symbol, dateString(plan.row.FromDate), to); err != nil {
		return fmt.Errorf("failed to store asset_symbol: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to store asset_symbol: %w", err)
	}
	return nil
}

// aliasPlan is the change StoreAlias makes: delete the same-ticker rows in
// remove, set the new to_date of the open rows in close, then insert row.
type aliasPlan struct {
	remove []time.Time
	close  []AliasInterval
	row    AliasInterval
}

// planAlias works out how in joins the stored rows of its ISIN. stored is
// ordered by from_date and has no overlaps, which the table guarantees.
func planAlias(stored []AliasInterval, in AliasInterval) (aliasPlan, error) {
	plan := aliasPlan{row: in}
	merged := make([]bool, len(stored))
	for grown := true; grown; {
		grown = false
		if plan.row.ToDate == nil {
			plan.row.ToDate = nextOtherTicker(stored, plan.row)
		}
		for i, s := range stored {
			if merged[i] || s.Symbol != plan.row.Symbol || !aliasesOverlap(s, plan.row) {
				continue
			}
			merged[i] = true
			grown = true
			plan.remove = append(plan.remove, s.FromDate)
			if s.FromDate.Before(plan.row.FromDate) {
				plan.row.FromDate = s.FromDate
			}
			plan.row.ToDate = laterKnownEnd(plan.row.ToDate, s.ToDate)
		}
	}

	for i, s := range stored {
		if merged[i] || !aliasesOverlap(s, plan.row) {
			continue
		}
		if s.ToDate == nil && s.FromDate.Before(plan.row.FromDate) {
			end := plan.row.FromDate
			s.ToDate = &end
			plan.close = append(plan.close, s)
			continue
		}
		return aliasPlan{}, fmt.Errorf("%w: %s is %s from %s, so it cannot also be %s from %s",
			ErrAliasConflict, in.ISIN, s.Symbol, dateString(s.FromDate), in.Symbol, dateString(in.FromDate))
	}
	return plan, nil
}

// nextOtherTicker is the from_date of the first stored row of another ticker
// that starts after row, or nil when there is none.
func nextOtherTicker(stored []AliasInterval, row AliasInterval) *time.Time {
	for _, s := range stored {
		if s.Symbol != row.Symbol && s.FromDate.After(row.FromDate) {
			from := s.FromDate
			return &from
		}
	}
	return nil
}

// laterKnownEnd is the later of two to_dates. An open end is unknown, not
// later, so a known end wins over it.
func laterKnownEnd(a, b *time.Time) *time.Time {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case b.After(*a):
		return b
	default:
		return a
	}
}

// aliasesOverlap reports whether two half-open [from, to) intervals share a day.
func aliasesOverlap(a, b AliasInterval) bool {
	aStartsBeforeBEnds := b.ToDate == nil || a.FromDate.Before(*b.ToDate)
	bStartsBeforeAEnds := a.ToDate == nil || b.FromDate.Before(*a.ToDate)
	return aStartsBeforeBEnds && bStartsBeforeAEnds
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
	return listAliases(ctx, db.Conn, isin, "")
}

func listAliases(ctx context.Context, conn postgres.Query, isin, lock string) ([]AliasInterval, error) {
	rows, err := conn.Query(ctx, `
		SELECT isin, symbol, from_date, to_date
		  FROM asset_symbol
		 WHERE isin = $1
		 ORDER BY from_date`+lock, isin)
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
