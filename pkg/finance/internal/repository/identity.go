package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/instrumentidentity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/priceunit"
)

// MigrateInstrumentIdentity creates instrument_identity from the statement
// backtest uses. It does not copy rows from asset_symbol. A venue source
// fills the rows later. Calling it twice is safe.
func MigrateInstrumentIdentity(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("instrument_identity: %w", err)
	}
	if _, err := conn.Exec(ctx, instrumentidentity.Table); err != nil {
		return fmt.Errorf("instrument_identity: %w", err)
	}
	if _, err := conn.Exec(ctx, instrumentidentity.Index); err != nil {
		return fmt.Errorf("instrument_identity: %w", err)
	}
	return nil
}

// Identity is one listing of an ISIN on a venue for a half-open window.
// Empty MIC, currency, or price unit means the column is null. ISIN is required.
type Identity struct {
	ID        int64
	ISIN      string
	MIC       string
	Currency  string
	PriceUnit string
	ValidFrom time.Time
	ValidTo   *time.Time
}

// NoIdentityError is a symbol that has no stored ISIN on the day asked.
// The text names the symbol. A lookup that does not need an identity does
// not return this.
type NoIdentityError struct {
	Symbol string
}

func (e *NoIdentityError) Error() string {
	return e.Symbol + " has no instrument identity"
}

// AmbiguousVenueError is one ISIN with more than one listing on the day.
// The caller does not pick a venue.
type AmbiguousVenueError struct {
	ISIN string
	Day  time.Time
}

func (e *AmbiguousVenueError) Error() string {
	return fmt.Sprintf("%s has more than one listing on %s", e.ISIN, e.Day.Format("2006-01-02"))
}

// Identities reads and writes instrument_identity.
type Identities struct {
	Conn postgres.Query
}

// Store inserts one identity row. An empty MIC, currency, or price unit is
// stored as null. Price unit is not filled from currency. The window is
// half-open; valid_to must be after valid_from.
func (db *Identities) Store(ctx context.Context, row Identity) (*Identity, error) {
	isin := strings.TrimSpace(row.ISIN)
	if isin == "" {
		return nil, ErrMissingISIN
	}
	if row.ValidFrom.IsZero() {
		return nil, errors.New("identity valid_from is missing")
	}
	from := dateOnly(row.ValidFrom)
	var to *time.Time
	if row.ValidTo != nil {
		end := dateOnly(*row.ValidTo)
		if !end.After(from) {
			return nil, errors.New("identity valid_to must be after valid_from")
		}
		to = &end
	}
	currency := strings.ToUpper(strings.TrimSpace(row.Currency))
	unit, err := priceunit.Canonical(currency, row.PriceUnit)
	if err != nil {
		return nil, err
	}
	var id int64
	err = db.Conn.QueryRow(ctx, `
		INSERT INTO instrument_identity (isin, mic, currency, price_unit, valid_from, valid_to)
		VALUES ($1, $2, $3, $4, $5::date, $6::date)
		RETURNING id`,
		isin, nullText(strings.ToUpper(row.MIC)),
		nullText(currency), nullText(unit),
		from.Format("2006-01-02"), dateArg(to),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	return db.byID(ctx, id)
}

// At returns the listing of isin on mic that covers on. An empty mic matches
// a row whose venue was never stored, not every venue. None is an error.
// Two rows is an error: the caller does not pick one.
func (db *Identities) At(ctx context.Context, isin, mic string, on time.Time) (*Identity, error) {
	isin = strings.TrimSpace(isin)
	mic = strings.ToUpper(strings.TrimSpace(mic))
	if isin == "" {
		return nil, ErrMissingISIN
	}
	on = dateOnly(on)
	rows, err := db.Conn.Query(ctx, `
		SELECT id, isin, mic, currency, price_unit, valid_from, valid_to
		  FROM instrument_identity
		 WHERE isin = $1
		   AND valid_from <= $2::date
		   AND (valid_to IS NULL OR $2::date < valid_to)
		   AND (($3 = '' AND mic IS NULL) OR mic = $3)
		 ORDER BY id`, isin, on.Format("2006-01-02"), mic)
	if err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	defer rows.Close()
	found, err := scanIdentities(rows)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		if mic == "" {
			return nil, fmt.Errorf("no listing for %s at %s", isin, on.Format("2006-01-02"))
		}
		return nil, fmt.Errorf("no listing for %s on %s at %s", isin, mic, on.Format("2006-01-02"))
	}
	if len(found) > 1 {
		return nil, &AmbiguousVenueError{ISIN: isin, Day: on}
	}
	row := found[0]
	return &row, nil
}

// BySymbol is the lookup that needs an identity. The symbol's ISIN on that
// day, from asset_symbol, selects the listing. A symbol with no stored ISIN
// fails with NoIdentityError and names the symbol. Nothing is guessed from
// the ticker, and a row in asset is not an identity.
func (db *Identities) BySymbol(ctx context.Context, symbol string, on time.Time) (*Identity, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol")
	}
	on = dateOnly(on)
	day := on.Format("2006-01-02")
	rows, err := db.Conn.Query(ctx, `
		SELECT DISTINCT i.isin
		  FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		 WHERE s.symbol = $1
		   AND s.from_date <= $2::date
		   AND (s.to_date IS NULL OR $2::date < s.to_date)`, symbol, day)
	if err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	defer rows.Close()
	var isins []string
	for rows.Next() {
		var isin string
		if err := rows.Scan(&isin); err != nil {
			return nil, fmt.Errorf("instrument_identity: %w", err)
		}
		isins = append(isins, isin)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	if len(isins) == 0 {
		return nil, &NoIdentityError{Symbol: symbol}
	}
	if len(isins) > 1 {
		return nil, fmt.Errorf("symbol %s maps to %d instruments at %s: %w",
			symbol, len(isins), day, ErrAmbiguousSymbol)
	}
	found, err := db.covering(ctx, isins[0], on)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, &NoIdentityError{Symbol: symbol}
	}
	if len(found) > 1 {
		return nil, &AmbiguousVenueError{ISIN: isins[0], Day: on}
	}
	row := found[0]
	return &row, nil
}

func (db *Identities) covering(ctx context.Context, isin string, on time.Time) ([]Identity, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT id, isin, mic, currency, price_unit, valid_from, valid_to
		  FROM instrument_identity
		 WHERE isin = $1
		   AND valid_from <= $2::date
		   AND (valid_to IS NULL OR $2::date < valid_to)
		 ORDER BY id`, isin, on.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	defer rows.Close()
	return scanIdentities(rows)
}

func (db *Identities) byID(ctx context.Context, id int64) (*Identity, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT id, isin, mic, currency, price_unit, valid_from, valid_to
		  FROM instrument_identity
		 WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	defer rows.Close()
	found, err := scanIdentities(rows)
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("instrument_identity: stored row %d is missing", id)
	}
	row := found[0]
	return &row, nil
}

func scanIdentities(rows pgx.Rows) ([]Identity, error) {
	out := make([]Identity, 0)
	for rows.Next() {
		var row Identity
		var isin, mic, currency, priceUnit *string
		var validTo *time.Time
		if err := rows.Scan(&row.ID, &isin, &mic, &currency, &priceUnit, &row.ValidFrom, &validTo); err != nil {
			return nil, fmt.Errorf("instrument_identity: %w", err)
		}
		row.ISIN = textOf(isin)
		row.MIC = textOf(mic)
		row.Currency = textOf(currency)
		row.PriceUnit = textOf(priceUnit)
		row.ValidFrom = dateOnly(row.ValidFrom)
		if validTo != nil {
			end := dateOnly(*validTo)
			row.ValidTo = &end
		}
		if row.ValidFrom.IsZero() {
			return nil, fmt.Errorf("instrument_identity: row %d has no valid_from", row.ID)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("instrument_identity: %w", err)
	}
	return out, nil
}

func nullText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func textOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func dateArg(day *time.Time) any {
	if day == nil {
		return nil
	}
	return day.Format("2006-01-02")
}
