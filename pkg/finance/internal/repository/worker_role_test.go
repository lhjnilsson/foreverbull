package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestWorkerRoleReadsBarsOnly(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})

	ctx := context.Background()
	owner, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(owner.Close)

	require.NoError(t, repository.CreateTables(ctx, owner))
	_, err = owner.Exec(ctx, `CREATE TABLE broker_secret (id int)`)
	require.NoError(t, err)
	_, err = owner.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ('AAA', 'Aaa')`)
	require.NoError(t, err)

	workerURL, err := environment.WorkerDatabaseURL()
	require.NoError(t, err)
	require.NotEqual(t, environment.GetPostgresURL(), workerURL)
	require.NotContains(t, workerURL, "postgres:foreverbull@")

	worker, err := pgxpool.New(ctx, workerURL)
	require.NoError(t, err)
	t.Cleanup(worker.Close)

	var name string
	require.NoError(t, worker.QueryRow(ctx, `SELECT name FROM asset WHERE symbol = 'AAA'`).Scan(&name))
	require.Equal(t, "Aaa", name)

	var n int
	require.NoError(t, worker.QueryRow(ctx, `SELECT count(*) FROM ohlc`).Scan(&n))
	require.NoError(t, worker.QueryRow(ctx, `SELECT count(*) FROM asset_symbol`).Scan(&n))

	_, err = worker.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ('BBB', 'Bbb')`)
	require.Equal(t, "42501", pgCode(t, err))

	err = worker.QueryRow(ctx, `SELECT id FROM broker_secret`).Scan(&n)
	require.Equal(t, "42501", pgCode(t, err))

	_, err = worker.Exec(ctx, `CREATE TABLE worker_made (id int)`)
	require.Equal(t, "42501", pgCode(t, err))
}

func pgCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)

	return pgErr.Code
}
