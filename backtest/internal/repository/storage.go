package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS period;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS portfolio;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS _order;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS position;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS execution_status;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS execution;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS session_status;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS session;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS backtest_status;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS backtest;`); err != nil {
		return err
	}

	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, BacktestTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, SessionTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, ExecutionTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, PeriodTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, OrderTable); err != nil {
		return err
	}
	CreateConstraint(ctx, conn)
	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, BacktestTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, SessionTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, ExecutionTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, PeriodTable); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, OrderTable); err != nil {
		return err
	}
	CreateConstraint(ctx, conn)
	return nil
}
