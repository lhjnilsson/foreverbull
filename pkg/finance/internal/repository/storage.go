package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS ohlc`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS asset`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return err
	}

	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return err
	}

	return nil
}
