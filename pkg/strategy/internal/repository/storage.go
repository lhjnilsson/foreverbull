package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Parent tables match pkg/service and pkg/backtest column types so FKs in
// Recreate tests (and CreateTables against a full broker DB) resolve.
const parentTables = `
CREATE TABLE IF NOT EXISTS service (
	image text PRIMARY KEY,
	algorithm JSONB,
	status int NOT NULL DEFAULT 0,
	error TEXT
);

CREATE TABLE IF NOT EXISTS backtest (
	name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
	benchmark text NOT NULL
);

CREATE TABLE IF NOT EXISTS session (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	backtest text REFERENCES backtest(name) ON DELETE CASCADE,
	port integer
);

CREATE TABLE IF NOT EXISTS backtest_execution (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	session text REFERENCES session(id) ON DELETE CASCADE,
	start_date date NOT NULL,
	end_date date NOT NULL
);
`

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	drops := []string{
		`DROP TABLE IF EXISTS strategy_deployment_run`,
		`DROP TABLE IF EXISTS strategy_deployment`,
		`DROP TABLE IF EXISTS strategy_experiment`,
		`DROP TABLE IF EXISTS strategy_version`,
		`DROP TABLE IF EXISTS strategy`,
	}
	for _, q := range drops {
		if _, err := conn.Exec(ctx, q); err != nil {
			return fmt.Errorf("failed to drop strategy table: %w", err)
		}
	}

	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		return fmt.Errorf("failed to create extension uuid-ossp: %w", err)
	}

	if _, err := conn.Exec(ctx, parentTables); err != nil {
		return fmt.Errorf("failed to create parent tables: %w", err)
	}

	if _, err := conn.Exec(ctx, `TRUNCATE TABLE service, backtest CASCADE`); err != nil {
		return fmt.Errorf("failed to truncate parent tables: %w", err)
	}

	return CreateTables(ctx, conn)
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		return fmt.Errorf("failed to create extension uuid-ossp: %w", err)
	}

	tables := []struct {
		name string
		sql  string
	}{
		{"strategy", StrategyTable},
		{"strategy_version", VersionTable},
		{"strategy_experiment", ExperimentTable},
		{"strategy_deployment", DeploymentTable},
		{"strategy_deployment_run", DeploymentRunTable},
	}
	for _, table := range tables {
		if _, err := conn.Exec(ctx, table.sql); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.name, err)
		}
	}

	return nil
}
