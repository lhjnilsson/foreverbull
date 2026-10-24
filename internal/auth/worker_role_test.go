package auth_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
)

func TestWorkerRoleReadsBarsAndNotAccounts(t *testing.T) {
	const password = "reader-pass-1"
	t.Setenv(environment.WorkerDBPassword, password)
	t.Setenv(environment.AuthEnabled, "true")
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `
		CREATE TABLE asset (symbol text primary key, name text);
		CREATE TABLE ohlc (
			symbol text,
			open numeric, high numeric, low numeric, close numeric,
			volume bigint, time timestamp, provider text
		);
		CREATE TABLE asset_symbol (symbol text)`)
	require.NoError(t, err)
	require.NoError(t, auth.EnsureWorkerRole(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))
	_, err = pool.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ('AAPL', 'Apple')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
		VALUES ('AAPL', 1, 4, 2, 3, 5, '2024-01-02', 'yahoo')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO asset_symbol (symbol) VALUES ('AAPL')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO account (role) VALUES ('admin')`)
	require.NoError(t, err)

	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	require.NoError(t, err)
	cfg.ConnConfig.User = "foreverbull_worker"
	cfg.ConnConfig.Password = password
	reader, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(reader.Close)

	var symbol string
	require.NoError(t, reader.QueryRow(ctx, `SELECT symbol FROM ohlc`).Scan(&symbol))
	require.Equal(t, "AAPL", symbol)
	require.NoError(t, reader.QueryRow(ctx, `SELECT symbol FROM asset`).Scan(&symbol))
	require.NoError(t, reader.QueryRow(ctx, `SELECT symbol FROM asset_symbol`).Scan(&symbol))
	require.Equal(t, "AAPL", symbol)

	err = reader.QueryRow(ctx, `SELECT id FROM account LIMIT 1`).Scan(&symbol)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "42501", pgErr.Code)
	require.NotContains(t, err.Error(), password)
	require.NotContains(t, err.Error(), environment.GetPostgresURL())
}

func TestWorkerRoleRejectsAnUnsafePassword(t *testing.T) {
	t.Setenv(environment.WorkerDBPassword, "has a space")
	err := auth.EnsureWorkerRole(context.Background(), nil)
	require.EqualError(t, err, "worker database password must be URL-safe")
	require.NotContains(t, err.Error(), "has a space")
}
