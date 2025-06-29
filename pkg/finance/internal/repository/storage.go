package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS trading_calendar_date`); err != nil {
		return fmt.Errorf("failed to drop trading_calendar_date table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS ohlc`); err != nil {
		return fmt.Errorf("failed to drop ohlc table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS asset`); err != nil {
		return fmt.Errorf("failed to drop asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS trading_session`); err != nil {
		return fmt.Errorf("failed to drop trading_session table: %w", err)
	}

	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("failed to create extension uuid-ossp: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return fmt.Errorf("failed to create asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return fmt.Errorf("failed to create ohlc table: %w", err)
	}

	if _, err := conn.Exec(ctx, TradingTable); err != nil {
		return fmt.Errorf("failed to create trading table: %w", err)
	}

	if _, err := conn.Exec(ctx, CalendarTable); err != nil {
		return fmt.Errorf("failed to create trading_calendar table: %w", err)
	}

	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("failed to create extension uuid-ossp: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return fmt.Errorf("failed to create asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return fmt.Errorf("failed to create ohlc table: %w", err)
	}

	if _, err := conn.Exec(ctx, TradingTable); err != nil {
		return fmt.Errorf("failed to create trading table: %w", err)
	}

	if _, err := conn.Exec(ctx, CalendarTable); err != nil {
		return fmt.Errorf("failed to create trading_calendar table: %w", err)
	}

	return nil
}
