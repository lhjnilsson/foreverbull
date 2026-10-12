package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Parent tables match pkg/service and pkg/backtest column types so FKs in
// Recreate tests (and CreateTables against a full broker DB) resolve.
// provider, profile, and function_arguments keep the real defaults, so an
// insert that omits them still lands.
const parentTables = `
CREATE TABLE IF NOT EXISTS service (
	image text PRIMARY KEY,
	algorithm JSONB,
	status int NOT NULL DEFAULT 0,
	error TEXT
);

CREATE TABLE IF NOT EXISTS backtest (
	name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
	benchmark text NOT NULL,
	provider text NOT NULL DEFAULT 'yahoo',
	profile text NOT NULL DEFAULT '',
	CONSTRAINT backtest_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca'))
);

CREATE TABLE IF NOT EXISTS session (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	backtest text NOT NULL REFERENCES backtest(name) ON DELETE CASCADE,
	port integer
);

CREATE TABLE IF NOT EXISTS backtest_execution (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	session text REFERENCES session(id) ON DELETE CASCADE,
	start_date date NOT NULL,
	end_date date NOT NULL,
	function_arguments jsonb,
	provider text NOT NULL DEFAULT 'yahoo',
	CONSTRAINT backtest_execution_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca'))
);

CREATE TABLE IF NOT EXISTS backtest_execution_period (
	id serial primary key,
	backtest_execution text not null,

	date date not null,
	pnl numeric  not null,
	returns numeric  not null,
	portfolio_value numeric  not null,

	longs_count integer,
	shorts_count integer,
	long_value numeric  not null,
	short_value numeric  not null,
	starting_exposure numeric  not null,
	ending_exposure numeric  not null,
	long_exposure numeric  not null,
	short_exposure numeric  not null,

	capital_used numeric  not null,
	gross_leverage numeric  not null,
	net_leverage numeric  not null,

	starting_value numeric  not null,
	ending_value numeric  not null,
	starting_cash numeric  not null,
	ending_cash numeric  not null,

	max_drawdown numeric  not null,
	max_leverage numeric  not null,
	excess_returns numeric  not null,
	treasury_period_return numeric  not null,
	algorithm_period_return numeric  not null,

	algo_volatility numeric,
	sharpe numeric,
	sortino numeric,

	benchmark_period_return numeric,
	benchmark_volatility numeric,
	alpha numeric,
	beta numeric,
	CONSTRAINT backtest_execution_period_backtest_execution_fkey
		FOREIGN KEY (backtest_execution) REFERENCES backtest_execution (id) ON DELETE CASCADE
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'unique_backtest_execution_period'
        AND conrelid = 'backtest_execution_period'::regclass
    ) THEN
        ALTER TABLE backtest_execution_period ADD CONSTRAINT unique_backtest_execution_period UNIQUE(backtest_execution, date);
    END IF;
END $$;
`

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	drops := []string{
		`DROP TABLE IF EXISTS strategy_deployment_run`,
		`DROP TABLE IF EXISTS strategy_deployment`,
		`DROP TABLE IF EXISTS strategy_experiment`,
		`DROP TABLE IF EXISTS strategy_version`,
		`DROP TABLE IF EXISTS strategy`,
		`DROP TABLE IF EXISTS backtest_execution_period`,
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

	return MigrateExperimentBacktest(ctx, conn)
}
