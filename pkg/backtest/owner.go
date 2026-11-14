package backtest

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
)

// IngestionCopier copies an ingestions object from the backtest name onto the id.
// Copy leaves the name key in place.
type IngestionCopier interface {
	Readable(ctx context.Context, key string) (bool, error)
	Copy(ctx context.Context, fromKey, toKey string) error
}

// MigrateOwner assigns backtest.owner_id, moves child keys onto backtest.id,
// and copies ingestion objects before the name primary key is dropped.
// Startup calls it when authentication is on, after the admin account exists.
func MigrateOwner(ctx context.Context, conn *pgxpool.Pool, adminID string, copier IngestionCopier) error {
	if copier == nil {
		return repository.MigrateBacktestOwner(ctx, conn, adminID, nil)
	}
	return repository.MigrateBacktestOwner(ctx, conn, adminID, ingestionAdapter{copier})
}

type ingestionAdapter struct {
	inner IngestionCopier
}

func (a ingestionAdapter) Readable(ctx context.Context, key string) (bool, error) {
	return a.inner.Readable(ctx, key)
}

func (a ingestionAdapter) Copy(ctx context.Context, fromKey, toKey string) error {
	return a.inner.Copy(ctx, fromKey, toKey)
}

// CopyNameKeyedIngestions ensures the schema, including the surrogate id,
// then copies each name-keyed ingestion onto that id. Auth-off startup runs
// it. The name key is left in place.
func CopyNameKeyedIngestions(ctx context.Context, conn *pgxpool.Pool, copier IngestionCopier) error {
	if err := repository.CreateTables(ctx, conn); err != nil {
		return err
	}
	if copier == nil {
		return repository.CopyIngestions(ctx, conn, nil)
	}
	return repository.CopyIngestions(ctx, conn, ingestionAdapter{copier})
}
