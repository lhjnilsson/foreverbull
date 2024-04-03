package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS strategy_execution_status;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS strategy_execution;`); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS strategy;`); err != nil {
		return err
	}
	return CreateTables(ctx, conn)
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("error creating uuid-ossp extension: %w", err)
	}
	if _, err := conn.Exec(ctx, StrategyTable); err != nil {
		return fmt.Errorf("error creating strategy table: %w", err)
	}
	if _, err := conn.Exec(ctx, ExecutionTable); err != nil {
		return fmt.Errorf("error creating execution table: %w", err)
	}
	return nil
}
