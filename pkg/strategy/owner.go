package strategy

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
)

// MigrateOwner assigns strategy.owner_id and replaces the global name unique
// with (owner_id, name). Startup calls it when authentication is on, after
// the service and backtest migrations.
func MigrateOwner(ctx context.Context, conn *pgxpool.Pool, adminID string) error {
	return repository.MigrateStrategyOwner(ctx, conn, adminID)
}
