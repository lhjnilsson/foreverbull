package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
)

func TestMigrateStrategyOwner(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))

	admin := insertStrategyAccount(t, pool, auth.RoleAdmin)
	editor := insertStrategyAccount(t, pool, auth.RoleEditor)
	_, err = pool.Exec(ctx, `INSERT INTO service (image) VALUES ('algo-a'), ('algo-b')`)
	require.NoError(t, err)

	var idA, idB string
	err = pool.QueryRow(ctx, `INSERT INTO strategy (name) VALUES ('mean-rev') RETURNING id`).Scan(&idA)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `INSERT INTO strategy (name) VALUES ('trend') RETURNING id`).Scan(&idB)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO strategy_version (strategy_id, rev, service_image)
		VALUES ($1, 1, 'algo-a'), ($2, 1, 'algo-b')`, idA, idB)
	require.NoError(t, err)

	err = repository.MigrateStrategyOwner(ctx, pool, "")
	require.ErrorContains(t, err, "strategy")
	require.ErrorContains(t, err, "admin account id")
	require.Equal(t, 2, strategyCount(t, pool, `SELECT COUNT(*) FROM strategy`))
	require.Equal(t, 2, strategyCount(t, pool, `SELECT COUNT(*) FROM strategy_version`))
	require.Equal(t, []string{"image"}, strategyPrimaryKey(t, pool, "service"))
	_, err = pool.Exec(ctx, `INSERT INTO strategy (name) VALUES ('mean-rev')`)
	require.Error(t, err)

	require.NoError(t, repository.MigrateStrategyOwner(ctx, pool, admin))
	require.NoError(t, repository.MigrateStrategyOwner(ctx, pool, admin))

	require.Equal(t, []string{"id"}, strategyPrimaryKey(t, pool, "strategy"))
	require.Equal(t, []string{"id"}, strategyPrimaryKey(t, pool, "service"))
	require.Equal(t, 0, strategyCount(t, pool, `SELECT COUNT(*) FROM account_password`))
	require.True(t, strategyColumn(t, pool, "strategy_version", "service_id"))
	require.True(t, strategyColumn(t, pool, "strategy_version", "service_image"))

	var ownerA, ownerB string
	err = pool.QueryRow(ctx, `SELECT owner_id FROM strategy WHERE id = $1`, idA).Scan(&ownerA)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT owner_id FROM strategy WHERE id = $1`, idB).Scan(&ownerB)
	require.NoError(t, err)
	require.Equal(t, admin, ownerA)
	require.Equal(t, admin, ownerB)
	require.NotEqual(t, editor, ownerA)

	var serviceID string
	err = pool.QueryRow(ctx, `
		SELECT version.service_id
		FROM strategy_version AS version
		JOIN service ON service.id = version.service_id
		WHERE version.strategy_id = $1 AND service.image = version.service_image`, idA).Scan(&serviceID)
	require.NoError(t, err)
	require.NotEmpty(t, serviceID)
	versionFK := strategyConstraint(t, pool, "strategy_version", "strategy_version_service_id_fkey")
	require.Contains(t, versionFK, "FOREIGN KEY (service_id) REFERENCES service(id)")
	refs, err := auth.ForeignKeysReferencing(ctx, pool, "service", "image")
	require.NoError(t, err)
	require.Empty(t, refs)

	_, err = pool.Exec(ctx, `INSERT INTO strategy (name, owner_id) VALUES ('mean-rev', $1)`, admin)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO strategy (name, owner_id) VALUES ('mean-rev', $1)`, editor)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO domain_grant (domain_kind, domain_id, grantee_id, level)
		VALUES ('strategy', $1, $2, 'editor')`, idB, editor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM strategy WHERE id = $1`, idB)
	require.NoError(t, err)
	require.Equal(t, 0, strategyCount(t, pool, `SELECT COUNT(*) FROM domain_grant WHERE domain_id = $1`, idB))
	require.Equal(t, 0, strategyCount(t, pool, `SELECT COUNT(*) FROM strategy_version WHERE strategy_id = $1`, idB))
}

func insertStrategyAccount(t *testing.T, pool *pgxpool.Pool, role string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO account (role) VALUES ($1) RETURNING id`, role).Scan(&id)
	require.NoError(t, err)
	return id
}

func strategyCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func strategyPrimaryKey(t *testing.T, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	columns, err := auth.PrimaryKeyColumns(context.Background(), pool, table)
	require.NoError(t, err)
	return columns
}

func strategyColumn(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	exists, err := auth.ColumnExists(context.Background(), pool, table, column)
	require.NoError(t, err)
	return exists
}

func strategyConstraint(t *testing.T, pool *pgxpool.Pool, table, name string) string {
	t.Helper()
	definition, ok, err := auth.ConstraintDefinition(context.Background(), pool, table, name)
	require.NoError(t, err)
	require.True(t, ok, name)
	return definition
}
