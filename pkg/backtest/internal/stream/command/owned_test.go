package command

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

type quietCopier struct{}

func (quietCopier) Readable(context.Context, string) (bool, error) { return false, nil }
func (quietCopier) Copy(context.Context, string, string) error {
	return context.Canceled
}

func TestLoadStoredBacktestRejectsForgedOwner(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))

	var ownerID string
	err = pool.QueryRow(ctx, `INSERT INTO account (role) VALUES ('admin') RETURNING id`).Scan(&ownerID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO account_password (account_id, username, password_hash)
		VALUES ($1, 'ada', 'x')`, ownerID)
	require.NoError(t, err)
	require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, ownerID, quietCopier{}))

	backtests := repository.Backtest{Conn: pool}
	_, err = backtests.CreateOwned(ctx, ownerID, &backtest.Backtest{Name: "spy"})
	require.NoError(t, err)
	var id string
	err = pool.QueryRow(ctx, `SELECT id FROM backtest WHERE owner_id = $1 AND name = 'spy'`, ownerID).Scan(&id)
	require.NoError(t, err)

	_, _, err = loadStoredBacktest(ctx, pool, "spy", "forged-owner", id)
	require.ErrorIs(t, err, errOwnerMismatch)
	require.NotContains(t, err.Error(), "SELECT")
	require.NotContains(t, err.Error(), id)
	require.NotContains(t, err.Error(), ownerID)

	item, gotID, err := loadStoredBacktest(ctx, pool, "spy", ownerID, id)
	require.NoError(t, err)
	require.Equal(t, id, gotID)
	require.Equal(t, "spy", item.GetName())
}
