package repository

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/instrumentidentity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// MigrateInstrumentIdentity creates instrument_identity from the statement
// finance uses. It does not copy rows from asset_symbol. Calling it twice
// is safe.
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
