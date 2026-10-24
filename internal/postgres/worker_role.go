package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
)

// WorkerReadTables are the only relations a strategy worker may read.
// Bar queries in the Python client use asset, ohlc, and asset_symbol.
var WorkerReadTables = []string{"asset", "ohlc", "asset_symbol"}

// EnsureWorkerRole creates the read-only login foreverbull_worker and grants
// SELECT on the bar tables in the current database. The password is
// WORKER_POSTGRES_PASSWORD, never the broker URL. Calling it again resets
// that role to the same privileges.
func EnsureWorkerRole(ctx context.Context, conn Query) error {
	secret, err := workerPassword()
	if err != nil {
		return err
	}

	role := pgx.Identifier{environment.WorkerPostgresRole}.Sanitize()
	if err := createWorkerRole(ctx, conn, role); err != nil {
		return err
	}
	if err := setWorkerPassword(ctx, conn, role, secret); err != nil {
		return err
	}
	if err := grantWorkerConnect(ctx, conn, role); err != nil {
		return err
	}
	return grantWorkerSelect(ctx, conn, role)
}

func workerPassword() (string, error) {
	raw, err := environment.WorkerDatabaseURL()
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse worker database URL: %w", err)
	}
	password, ok := parsed.User.Password()
	if !ok || password == "" {
		return "", fmt.Errorf("%s is empty", environment.WorkerPostgresPassword)
	}
	return password, nil
}

func createWorkerRole(ctx context.Context, conn Query, role string) error {
	// The role name is the package constant, sanitized as an identifier above.
	statement := `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'foreverbull_worker') THEN
    CREATE ROLE ` + role + ` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
  END IF;
END
$$`
	if _, err := conn.Exec(ctx, statement); err != nil {
		return fmt.Errorf("create role %s: %w", environment.WorkerPostgresRole, err)
	}
	return nil
}

func setWorkerPassword(ctx context.Context, conn Query, role, secret string) error {
	var literal string
	if err := conn.QueryRow(ctx, `SELECT quote_literal($1::text)`, secret).Scan(&literal); err != nil {
		return fmt.Errorf("quote worker password: %w", err)
	}
	statement := `ALTER ROLE ` + role + ` WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD ` + literal
	if _, err := conn.Exec(ctx, statement); err != nil {
		return fmt.Errorf("set password for %s: %w", environment.WorkerPostgresRole, err)
	}
	return nil
}

func grantWorkerConnect(ctx context.Context, conn Query, role string) error {
	var dbname string
	if err := conn.QueryRow(ctx, `SELECT current_database()`).Scan(&dbname); err != nil {
		return fmt.Errorf("current database: %w", err)
	}
	database := pgx.Identifier{dbname}.Sanitize()
	if _, err := conn.Exec(ctx, `GRANT CONNECT ON DATABASE `+database+` TO `+role); err != nil {
		return fmt.Errorf("grant connect on %s: %w", dbname, err)
	}
	if _, err := conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+role); err != nil {
		return fmt.Errorf("grant usage on schema public: %w", err)
	}
	// Postgres 13 grants CREATE on schema public to PUBLIC. A worker login
	// would inherit that and could create tables next to the broker's.
	// The broker session keeps CREATE so later migrations still run.
	if _, err := conn.Exec(ctx, `REVOKE CREATE ON SCHEMA public FROM PUBLIC`); err != nil {
		return fmt.Errorf("revoke create on schema public: %w", err)
	}
	if _, err := conn.Exec(ctx, `GRANT CREATE ON SCHEMA public TO current_user`); err != nil {
		return fmt.Errorf("grant create on schema public to current user: %w", err)
	}
	return nil
}

func grantWorkerSelect(ctx context.Context, conn Query, role string) error {
	if _, err := conn.Exec(ctx, `REVOKE ALL ON ALL TABLES IN SCHEMA public FROM `+role); err != nil {
		return fmt.Errorf("revoke table privileges from %s: %w", environment.WorkerPostgresRole, err)
	}
	tables := make([]string, len(WorkerReadTables))
	for i, name := range WorkerReadTables {
		tables[i] = pgx.Identifier{name}.Sanitize()
	}
	statement := `GRANT SELECT ON TABLE ` + strings.Join(tables, ", ") + ` TO ` + role
	if _, err := conn.Exec(ctx, statement); err != nil {
		return fmt.Errorf("grant select to %s: %w", environment.WorkerPostgresRole, err)
	}
	return nil
}
