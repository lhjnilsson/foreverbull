package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
)

func TestMigrateServiceOwner(t *testing.T) {
	pool := newServicePool(t)
	ctx := context.Background()
	require.NoError(t, repository.CreateTables(ctx, pool))
	require.NoError(t, auth.Migrate(ctx, pool))

	admin := insertAccount(t, pool, auth.RoleAdmin)
	editor := insertAccount(t, pool, auth.RoleEditor)
	_, err := pool.Exec(ctx, `INSERT INTO service (image) VALUES ('algo-a'), ('algo-b')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO service_instance (id, image) VALUES ('inst-a', 'algo-a')`)
	require.NoError(t, err)

	err = repository.MigrateServiceOwner(ctx, pool, "")
	require.ErrorContains(t, err, "service")
	require.ErrorContains(t, err, "admin account id")
	require.Equal(t, 2, countQuery(t, pool, `SELECT COUNT(*) FROM service`))
	require.Equal(t, []string{"image"}, primaryKey(t, pool, "service"))
	require.Equal(t, 0, countQuery(t, pool, `SELECT COUNT(*) FROM account_password`))

	err = repository.MigrateServiceOwner(ctx, pool, "not-an-account")
	require.ErrorContains(t, err, "service")
	require.ErrorContains(t, err, "missing")
	require.Equal(t, 2, countQuery(t, pool, `SELECT COUNT(*) FROM service`))
	require.Equal(t, []string{"image"}, primaryKey(t, pool, "service"))

	require.NoError(t, repository.MigrateServiceOwner(ctx, pool, admin))
	require.NoError(t, repository.MigrateServiceOwner(ctx, pool, admin))

	require.Equal(t, []string{"id"}, primaryKey(t, pool, "service"))
	require.True(t, columnExists(t, pool, "service_status", "service_id"))
	require.False(t, columnExists(t, pool, "service_status", "image"))
	require.Equal(t, 0, countQuery(t, pool, `SELECT COUNT(*) FROM account_password`))

	var idA, ownerA, statusA, instanceA string
	err = pool.QueryRow(ctx, `SELECT id, owner_id FROM service WHERE image = 'algo-a'`).Scan(&idA, &ownerA)
	require.NoError(t, err)
	require.Equal(t, admin, ownerA)
	require.NotEqual(t, editor, ownerA)
	err = pool.QueryRow(ctx, `SELECT service_id FROM service_status WHERE service_id = $1`, idA).Scan(&statusA)
	require.NoError(t, err)
	require.Equal(t, idA, statusA)
	err = pool.QueryRow(ctx, `SELECT service_id FROM service_instance WHERE id = 'inst-a'`).Scan(&instanceA)
	require.NoError(t, err)
	require.Equal(t, idA, instanceA)

	var ownerB, idB string
	err = pool.QueryRow(ctx, `SELECT id, owner_id FROM service WHERE image = 'algo-b'`).Scan(&idB, &ownerB)
	require.NoError(t, err)
	require.Equal(t, admin, ownerB)
	require.NotEmpty(t, idB)
	require.NotEqual(t, idA, idB)

	statusFK := constraintDef(t, pool, "service_status", "service_status_service_id_fkey")
	require.Contains(t, statusFK, "FOREIGN KEY (service_id) REFERENCES service(id)")
	require.Contains(t, statusFK, "ON DELETE CASCADE")
	instanceFK := constraintDef(t, pool, "service_instance", "service_instance_service_id_fkey")
	require.Contains(t, instanceFK, "FOREIGN KEY (service_id) REFERENCES service(id)")
	require.NotContains(t, instanceFK, "ON DELETE CASCADE")

	var definition string
	err = pool.QueryRow(ctx, `SELECT pg_get_functiondef('notify_service_status()'::regprocedure)`).Scan(&definition)
	require.NoError(t, err)
	require.Contains(t, definition, "INSERT INTO service_status (service_id, status, error)")
	require.Contains(t, definition, "pg_notify('service_status', NEW.id)")
	require.NotContains(t, definition, "service_status (image, status, error)")

	_, err = pool.Exec(ctx, `INSERT INTO service (image, owner_id) VALUES ('algo-a', $1)`, admin)
	require.Error(t, err)
	var other string
	err = pool.QueryRow(ctx, `
		INSERT INTO service (image, owner_id) VALUES ('algo-a', $1) RETURNING id`, editor).Scan(&other)
	require.NoError(t, err)
	require.Equal(t, 1, countQuery(t, pool, `SELECT COUNT(*) FROM service_status WHERE service_id = $1`, other))

	_, err = pool.Exec(ctx, `
		INSERT INTO domain_grant (domain_kind, domain_id, grantee_id, level)
		VALUES ('service', $1, $2, 'viewer')`, idB, editor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM service WHERE id = $1`, idB)
	require.NoError(t, err)
	require.Equal(t, 0, countQuery(t, pool, `SELECT COUNT(*) FROM domain_grant WHERE domain_id = $1`, idB))
	require.Equal(t, 0, countQuery(t, pool, `SELECT COUNT(*) FROM service_status WHERE service_id = $1`, idB))

	// CreateTables still installs the image notify function. A later owner
	// migration puts the id insert back.
	require.NoError(t, repository.CreateTables(ctx, pool))
	require.NoError(t, repository.MigrateServiceOwner(ctx, pool, admin))
	var repaired string
	err = pool.QueryRow(ctx, `
		INSERT INTO service (image, owner_id) VALUES ('algo-d', $1) RETURNING id`, admin).Scan(&repaired)
	require.NoError(t, err)
	require.Equal(t, 1, countQuery(t, pool, `SELECT COUNT(*) FROM service_status WHERE service_id = $1`, repaired))
}

func newServicePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func insertAccount(t *testing.T, pool *pgxpool.Pool, role string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO account (role) VALUES ($1) RETURNING id`, role).Scan(&id)
	require.NoError(t, err)
	return id
}

func countQuery(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func primaryKey(t *testing.T, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	columns, err := auth.PrimaryKeyColumns(context.Background(), pool, table)
	require.NoError(t, err)
	return columns
}

func columnExists(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	exists, err := auth.ColumnExists(context.Background(), pool, table, column)
	require.NoError(t, err)
	return exists
}

func constraintDef(t *testing.T, pool *pgxpool.Pool, table, name string) string {
	t.Helper()
	definition, ok, err := auth.ConstraintDefinition(context.Background(), pool, table, name)
	require.NoError(t, err)
	require.True(t, ok, name)
	return definition
}
