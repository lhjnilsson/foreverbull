package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const workerRole = "foreverbull_worker"

// EnsureWorkerRole creates the reader role when WORKER_DB_PASSWORD is set.
// An empty password leaves the database unchanged. The role can SELECT asset
// and ohlc. It is not granted account, api_token, browser_session, oauth_state,
// or domain_grant. The password is not included in the error text.
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
	}
	for _, grant := range grants {
		if _, err := conn.Exec(ctx, grant); err != nil {
			return fmt.Errorf("worker role grant: %w", err)
		}
	}
	return nil
}
