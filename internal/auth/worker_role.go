package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const workerRole = "foreverbull_worker"

// workerRoleLockKey serializes role updates. The login is cluster-wide, and
// two ALTER ROLE statements on it return SQLSTATE XX000.
const workerRoleLockKey int64 = 0x6662776b

// EnsureWorkerRole creates the reader role when WORKER_DB_PASSWORD is set.
// An empty password leaves the database unchanged. The role can SELECT asset,
// ohlc, and asset_symbol. It is not granted account, api_token, browser_session,
// oauth_state, or domain_grant. The password is not included in the error text.
func EnsureWorkerRole(ctx context.Context, conn postgres.Query) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	password := os.Getenv(environment.WorkerDBPassword)
	if password == "" {
		return nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9._~-]+$`).MatchString(password) {
		return errors.New("worker database password must be URL-safe")
	}

	beginner, ok := conn.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return applyWorkerRole(ctx, conn, password)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin worker role: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// The lock ends with the transaction, including when the connection is pooled.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, workerRoleLockKey); err != nil {
		return fmt.Errorf("lock worker role: %w", err)
	}
	if err := applyWorkerRole(ctx, tx, password); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit worker role: %w", err)
	}
	return nil
}

func applyWorkerRole(ctx context.Context, conn postgres.Query, password string) error {
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, workerRole).Scan(&exists); err != nil {
		return fmt.Errorf("worker role lookup: %w", err)
	}
	query := `SELECT format('CREATE ROLE foreverbull_worker LOGIN PASSWORD %L', $1::text)`
	if exists {
		query = `SELECT format('ALTER ROLE foreverbull_worker WITH LOGIN PASSWORD %L', $1::text)`
	}
	var stmt string
	if err := conn.QueryRow(ctx, query, password).Scan(&stmt); err != nil {
		return fmt.Errorf("worker role statement: %w", err)
	}
	// stmt is Postgres format %L of a URL-safe password, not caller SQL.
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("worker role: %w", err)
	}
	grants := []string{
		`GRANT USAGE ON SCHEMA public TO foreverbull_worker`,
		`GRANT SELECT ON TABLE asset TO foreverbull_worker`,
		`GRANT SELECT ON TABLE ohlc TO foreverbull_worker`,
		`GRANT SELECT ON TABLE asset_symbol TO foreverbull_worker`,
	}
	for _, grant := range grants {
		if _, err := conn.Exec(ctx, grant); err != nil {
			return fmt.Errorf("worker role grant: %w", err)
		}
	}
	return nil
}
