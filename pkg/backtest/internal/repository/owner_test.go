package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
)

func TestMigrateBacktestOwner(t *testing.T) {
	pool := newBacktestPool(t)
	ctx := context.Background()

	t.Run("missing admin deletes nothing", func(t *testing.T) {
		resetBacktests(t, pool)
		seedBacktests(t, pool)
		err := repository.MigrateBacktestOwner(ctx, pool, "", nil)
		require.ErrorContains(t, err, "backtest")
		require.ErrorContains(t, err, "admin account id")
		require.Equal(t, 2, countBacktestRows(t, pool, `SELECT COUNT(*) FROM backtest`))
		require.Equal(t, []string{"name"}, backtestPrimaryKey(t, pool, "backtest"))
		require.Equal(t, 0, countBacktestRows(t, pool, `SELECT COUNT(*) FROM account_password`))
	})

	t.Run("assigns owners and copies ingestion objects", func(t *testing.T) {
		resetBacktests(t, pool)
		seedBacktests(t, pool)
		admin := insertBacktestAccount(t, pool, auth.RoleAdmin)
		editor := insertBacktestAccount(t, pool, auth.RoleEditor)
		sessions := sessionNames(t, pool)
		copier := &memIngest{keys: map[string]bool{"spy": true, "qqq": true}}

		require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, copier))
		copies := copier.copies
		require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, copier))
		require.Equal(t, copies, copier.copies)
		require.Equal(t, 2, countBacktestRows(t, pool, `SELECT COUNT(*) FROM backtest`))
		require.Equal(t, 0, countBacktestRows(t, pool, `SELECT COUNT(*) FROM account_password`))
		require.Equal(t, []string{"id"}, backtestPrimaryKey(t, pool, "backtest"))
		require.False(t, backtestColumn(t, pool, "session", "backtest"))
		require.False(t, backtestColumn(t, pool, "backtest_period", "backtest_name"))
		require.False(t, backtestColumn(t, pool, "strategy_experiment", "backtest_name"))
		require.True(t, copier.keys["spy"])
		require.True(t, copier.keys["qqq"])

		var spyID, qqqID string
		for _, name := range []string{"spy", "qqq"} {
			var id, owner string
			err := pool.QueryRow(ctx, `SELECT id, owner_id FROM backtest WHERE name = $1`, name).Scan(&id, &owner)
			require.NoError(t, err)
			require.Equal(t, admin, owner)
			require.NotEqual(t, editor, owner)
			require.True(t, copier.keys[id], name)
			if name == "spy" {
				spyID = id
			} else {
				qqqID = id
			}
		}
		require.NotEqual(t, spyID, qqqID)

		for sessionID, name := range sessions {
			var got string
			err := pool.QueryRow(ctx, `
				SELECT backtest.name
				FROM session
				JOIN backtest ON backtest.id = session.backtest_id
				WHERE session.id = $1`, sessionID).Scan(&got)
			require.NoError(t, err)
			require.Equal(t, name, got)
		}

		sessionFK := backtestConstraint(t, pool, "session", "session_backtest_id_fkey")
		require.Contains(t, sessionFK, "FOREIGN KEY (backtest_id) REFERENCES backtest(id)")
		require.Contains(t, sessionFK, "ON DELETE CASCADE")
		periodFK := backtestConstraint(t, pool, "backtest_period", "backtest_period_backtest_id_fkey")
		require.Contains(t, periodFK, "ON DELETE CASCADE")
		assetFK := backtestConstraint(t, pool, "backtest_period_asset", "backtest_period_asset_period_fkey")
		require.Contains(t, assetFK, "FOREIGN KEY (backtest_id, start_date, end_date)")
		experimentFK := backtestConstraint(t, pool, "strategy_experiment", "strategy_experiment_backtest_id_fkey")
		require.Contains(t, experimentFK, "REFERENCES backtest(id)")
		require.Equal(t, []string{"backtest_id", "isin", "from_date"}, backtestPrimaryKey(t, pool, "universe_member"))
		exclusion := backtestConstraint(t, pool, "universe_member", "universe_member_no_overlap")
		require.Contains(t, exclusion, "backtest_id")
		require.NotContains(t, exclusion, "backtest_name")

		require.Equal(t, 1, countBacktestRows(t, pool, `SELECT COUNT(*) FROM backtest_execution`))
		_, err := pool.Exec(ctx, `INSERT INTO backtest (name, benchmark, provider, owner_id) VALUES ('spy', 'SPY', 'yahoo', $1)`, admin)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO backtest (name, benchmark, provider, owner_id) VALUES ('spy', 'SPY', 'yahoo', $1)`, editor)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO backtest (name, benchmark, provider, owner_id) VALUES ('', 'SPY', 'yahoo', $1)`, editor)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO backtest (name, benchmark, provider, owner_id) VALUES ('bad', 'SPY', 'nasdaq', $1)`, editor)
		require.Error(t, err)

		require.NoError(t, repository.CreateTables(ctx, pool))
		require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, copier))
	})

	t.Run("failed copy keeps the name key", func(t *testing.T) {
		resetBacktests(t, pool)
		seedBacktests(t, pool)
		admin := insertBacktestAccount(t, pool, auth.RoleAdmin)
		copier := &memIngest{
			keys:    map[string]bool{"spy": true, "qqq": true},
			copyErr: fmt.Errorf("disk"),
		}
		err := repository.MigrateBacktestOwner(ctx, pool, admin, copier)
		require.ErrorContains(t, err, "ingestions")
		require.Equal(t, 2, countBacktestRows(t, pool, `SELECT COUNT(*) FROM backtest`))
		require.Equal(t, []string{"name"}, backtestPrimaryKey(t, pool, "backtest"))
		require.True(t, copier.keys["spy"])
		require.True(t, copier.keys["qqq"])
		require.Len(t, copier.keys, 2)

		copier.copyErr = nil
		require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, copier))
		require.Equal(t, []string{"id"}, backtestPrimaryKey(t, pool, "backtest"))
		require.True(t, copier.keys["spy"])
		require.True(t, copier.keys["qqq"])
		var id string
		err = pool.QueryRow(ctx, `SELECT id FROM backtest WHERE name = 'spy'`).Scan(&id)
		require.NoError(t, err)
		require.True(t, copier.keys[id])
	})
}

type memIngest struct {
	keys    map[string]bool
	copies  int
	copyErr error
}

func (m *memIngest) Readable(_ context.Context, key string) (bool, error) {
	return m.keys[key], nil
}

func (m *memIngest) Copy(_ context.Context, fromKey, toKey string) error {
	if m.copyErr != nil {
		return m.copyErr
	}
	if !m.keys[fromKey] {
		return fmt.Errorf("missing %s", fromKey)
	}
	m.keys[toKey] = true
	m.copies++
	return nil
}

func newBacktestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func resetBacktests(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `DROP TABLE IF EXISTS strategy_experiment`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DROP TABLE IF EXISTS strategy`)
	require.NoError(t, err)
	require.NoError(t, repository.Recreate(ctx, pool))
	_, err = pool.Exec(ctx, `
		CREATE TABLE strategy (
			id text PRIMARY KEY DEFAULT uuid_generate_v4(),
			name text NOT NULL UNIQUE
		);
		CREATE TABLE strategy_experiment (
			id text PRIMARY KEY DEFAULT uuid_generate_v4(),
			strategy_id text NOT NULL REFERENCES strategy (id) ON DELETE CASCADE,
			backtest_name text NOT NULL,
			CONSTRAINT strategy_experiment_backtest_name_fkey
				FOREIGN KEY (backtest_name) REFERENCES backtest (name)
		)`)
	require.NoError(t, err)
}

func seedBacktests(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO backtest (name, benchmark, provider) VALUES
			('spy', 'SPY', 'yahoo'),
			('qqq', 'QQQ', 'stooq')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO backtest_period (backtest_name, start_date, end_date) VALUES
			('spy', '2020-01-01', '2020-12-31'),
			('qqq', '2021-01-01', '2021-12-31')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO backtest_period_asset (backtest_name, start_date, end_date, symbol, enabled) VALUES
			('spy', '2020-01-01', '2020-12-31', 'AAPL', true),
			('qqq', '2021-01-01', '2021-12-31', 'MSFT', true)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO session (backtest) VALUES ('spy'), ('qqq')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO backtest_execution (session, start_date, end_date, provider)
		SELECT id, '2020-01-01', '2020-12-31', 'yahoo' FROM session WHERE backtest = 'spy'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO instrument (id, isin, name) VALUES ('inst-1', 'US0378331005', 'Apple')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO universe_member (backtest_name, isin, from_date) VALUES
			('spy', 'US0378331005', '2020-01-01'),
			('qqq', 'US0378331005', '2020-01-01')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO strategy (name) VALUES ('mean-rev')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO strategy_experiment (strategy_id, backtest_name)
		SELECT id, 'spy' FROM strategy
		UNION ALL
		SELECT id, 'qqq' FROM strategy`)
	require.NoError(t, err)
}

func sessionNames(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id, backtest FROM session`)
	require.NoError(t, err)
	defer rows.Close()
	names := map[string]string{}
	for rows.Next() {
		var id, name string
		require.NoError(t, rows.Scan(&id, &name))
		names[id] = name
	}
	require.NoError(t, rows.Err())
	require.Len(t, names, 2)
	return names
}

func insertBacktestAccount(t *testing.T, pool *pgxpool.Pool, role string) string {
	t.Helper()
	require.NoError(t, auth.Migrate(context.Background(), pool))
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO account (role) VALUES ($1) RETURNING id`, role).Scan(&id)
	require.NoError(t, err)
	return id
}

func countBacktestRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func backtestPrimaryKey(t *testing.T, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	columns, err := auth.PrimaryKeyColumns(context.Background(), pool, table)
	require.NoError(t, err)
	return columns
}

func backtestColumn(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	exists, err := auth.ColumnExists(context.Background(), pool, table, column)
	require.NoError(t, err)
	return exists
}

func backtestConstraint(t *testing.T, pool *pgxpool.Pool, table, name string) string {
	t.Helper()
	definition, ok, err := auth.ConstraintDefinition(context.Background(), pool, table, name)
	require.NoError(t, err)
	require.True(t, ok, name)
	return definition
}
