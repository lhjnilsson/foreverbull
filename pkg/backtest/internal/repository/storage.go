package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error { //nolint: cyclop
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS period;`); err != nil {
		return fmt.Errorf("failed to drop table period: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS portfolio;`); err != nil {
		return fmt.Errorf("failed to drop table portfolio: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS _order;`); err != nil {
		return fmt.Errorf("failed to drop table _order: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS position;`); err != nil {
		return fmt.Errorf("failed to drop table position: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS execution_status;`); err != nil {
		return fmt.Errorf("failed to drop table execution_status: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS execution;`); err != nil {
		return fmt.Errorf("failed to drop table execution: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS session_status;`); err != nil {
		return fmt.Errorf("failed to drop table session_status: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS session;`); err != nil {
		return fmt.Errorf("failed to drop table session: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS backtest_status;`); err != nil {
		return fmt.Errorf("failed to drop table backtest_status: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS backtest;`); err != nil {
		return fmt.Errorf("failed to drop table backtest: %w", err)
	}

	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("failed to create extension uuid-ossp: %w", err)
	}

	if _, err := conn.Exec(ctx, BacktestTable); err != nil {
		return fmt.Errorf("failed to create table backtest: %w", err)
	}

	if _, err := conn.Exec(ctx, SessionTable); err != nil {
		return fmt.Errorf("failed to create table session: %w", err)
	}

	if _, err := conn.Exec(ctx, ExecutionTable); err != nil {
		return fmt.Errorf("failed to create table execution: %w", err)
	}

	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("failed to create extension uuid-ossp: %w", err)
	}

	if _, err := conn.Exec(ctx, BacktestTable); err != nil {
		return fmt.Errorf("failed to create table backtest: %w", err)
	}

	if _, err := conn.Exec(ctx, SessionTable); err != nil {
		return fmt.Errorf("failed to create table session: %w", err)
	}

	if _, err := conn.Exec(ctx, ExecutionTable); err != nil {
		return fmt.Errorf("failed to create table execution: %w", err)
	}

	return nil
}
