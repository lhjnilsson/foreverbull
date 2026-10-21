package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
)

// MigrateOwner assigns service.owner_id and moves the primary key to id.
// Strategy migration calls this before it backfills strategy_version.service_id.
// Calling it twice is safe. The broker startup path does not call it.
func MigrateOwner(ctx context.Context, conn *pgxpool.Pool, adminID string) error {
	return repository.MigrateServiceOwner(ctx, conn, adminID)
}
