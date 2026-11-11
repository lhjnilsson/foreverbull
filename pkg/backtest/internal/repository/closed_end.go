package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// ErrNoSessionCutoff is a backtest row whose download did not record the
// session it stops at. Ingest cannot choose one by reading the clock.
var ErrNoSessionCutoff = errors.New("no session cut-off was recorded")

const addClosedEnd = `
ALTER TABLE backtest
	ADD COLUMN IF NOT EXISTS closed_end date`

// MigrateClosedEnd adds backtest.closed_end, the capped period end from the
// download's one clock reading. Calling it twice is safe. A null value is a
// backtest whose download did not record one.
func MigrateClosedEnd(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, addClosedEnd); err != nil {
		return fmt.Errorf("backtest: closed_end: %w", err)
	}
	return nil
}

// SaveClosedEnd records the capped period end of the backtest called name.
// A backtest that isn't stored is pgx.ErrNoRows.
func (db *Backtest) SaveClosedEnd(ctx context.Context, name string, end time.Time) error {
	if authOn() {
		return fmt.Errorf("save session cut-off by name is refused while AUTH_ENABLED=true")
	}
	return db.saveClosedEnd(ctx, `UPDATE backtest SET closed_end = $2 WHERE name = $1`, name, end)
}

// SaveClosedEndByID is SaveClosedEnd for one backtest id.
func (db *Backtest) SaveClosedEndByID(ctx context.Context, id string, end time.Time) error {
	if id == "" {
		return errors.New("backtest id is empty")
	}
	return db.saveClosedEnd(ctx, `UPDATE backtest SET closed_end = $2 WHERE id = $1`, id, end)
}

func (db *Backtest) saveClosedEnd(ctx context.Context, query, key string, end time.Time) error {
	tag, err := db.Conn.Exec(ctx, query, key, end.UTC().Format(time.DateOnly))
	if err != nil {
		return fmt.Errorf("failed to store session cut-off: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ClosedEnd is the capped period end recorded for name, as midnight UTC.
// A missing backtest is pgx.ErrNoRows. A row with no cut-off is
// ErrNoSessionCutoff.
func (db *Backtest) ClosedEnd(ctx context.Context, name string) (time.Time, error) {
	if authOn() {
		return time.Time{}, fmt.Errorf("read session cut-off by name is refused while AUTH_ENABLED=true")
	}
	return db.closedEnd(ctx, `SELECT closed_end FROM backtest WHERE name = $1`, name)
}

// ClosedEndByID is ClosedEnd for one backtest id.
func (db *Backtest) ClosedEndByID(ctx context.Context, id string) (time.Time, error) {
	if id == "" {
		return time.Time{}, errors.New("backtest id is empty")
	}
	return db.closedEnd(ctx, `SELECT closed_end FROM backtest WHERE id = $1`, id)
}

func (db *Backtest) closedEnd(ctx context.Context, query, key string) (time.Time, error) {
	var closed *time.Time
	err := db.Conn.QueryRow(ctx, query, key).Scan(&closed)
	if err != nil {
		return time.Time{}, err
	}
	if closed == nil {
		return time.Time{}, ErrNoSessionCutoff
	}
	year, month, day := closed.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
}
