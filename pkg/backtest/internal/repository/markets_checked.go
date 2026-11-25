package repository

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// markets_checked records that create or ingest already accepted every
// symbol and the benchmark as US-listed in USD. A run reads the flag and
// does not ask the provider again.
const addMarketsChecked = `
ALTER TABLE backtest
	ADD COLUMN IF NOT EXISTS markets_checked boolean NOT NULL DEFAULT false`

// MigrateMarketsChecked adds backtest.markets_checked. Calling it twice is safe.
func MigrateMarketsChecked(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, addMarketsChecked); err != nil {
		return fmt.Errorf("backtest: markets_checked: %w", err)
	}
	return nil
}

// MarkMarketsChecked records that the listing check passed.
func (db *Backtest) MarkMarketsChecked(ctx context.Context, name, id string) error {
	if environment.GetAuthEnabled() {
		if id == "" {
			return fmt.Errorf("backtest id is empty")
		}
		_, err := db.Conn.Exec(ctx, `UPDATE backtest SET markets_checked = true WHERE id = $1`, id)
		return err
	}
	_, err := db.Conn.Exec(ctx, `UPDATE backtest SET markets_checked = true WHERE name = $1`, name)
	return err
}

// MarketsChecked reports whether create or ingest already accepted the listings.
func (db *Backtest) MarketsChecked(ctx context.Context, name, id string) (bool, error) {
	var checked bool
	var err error
	if environment.GetAuthEnabled() {
		err = db.Conn.QueryRow(ctx, `SELECT markets_checked FROM backtest WHERE id = $1`, id).Scan(&checked)
	} else {
		err = db.Conn.QueryRow(ctx, `SELECT markets_checked FROM backtest WHERE name = $1`, name).Scan(&checked)
	}
	if err != nil {
		return false, err
	}
	return checked, nil
}
