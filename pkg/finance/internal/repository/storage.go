package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS ohlc`); err != nil {
		return fmt.Errorf("failed to drop ohlc table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS asset`); err != nil {
		return fmt.Errorf("failed to drop asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return fmt.Errorf("failed to create asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return fmt.Errorf("failed to create ohlc table: %w", err)
	}

	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return fmt.Errorf("failed to create asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return fmt.Errorf("failed to create ohlc table: %w", err)
	}

	return nil
}
