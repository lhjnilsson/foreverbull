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

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS corporate_action`); err != nil {
		return fmt.Errorf("failed to drop corporate_action table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS dividend_fetch`); err != nil {
		return fmt.Errorf("failed to drop dividend_fetch table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS dividend`); err != nil {
		return fmt.Errorf("failed to drop dividend table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP VIEW IF EXISTS instrument_ohlc`); err != nil {
		return fmt.Errorf("failed to drop instrument_ohlc view: %w", err)
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS provider_symbol`); err != nil {
		return fmt.Errorf("failed to drop provider_symbol table: %w", err)
	}
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS ohlc`); err != nil {
		return fmt.Errorf("failed to drop ohlc table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS asset_symbol`); err != nil {
		return fmt.Errorf("failed to drop asset_symbol table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS instrument`); err != nil {
		return fmt.Errorf("failed to drop instrument table: %w", err)
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

	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("failed to create extension btree_gist: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return fmt.Errorf("failed to create asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, InstrumentTable); err != nil {
		return fmt.Errorf("failed to create instrument table: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetSymbolTable); err != nil {
		return fmt.Errorf("failed to create asset_symbol table: %w", err)
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return fmt.Errorf("failed to create ohlc table: %w", err)
	}

	if err := migrateFinanceSchema(ctx, conn); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, DividendTable); err != nil {
		return fmt.Errorf("failed to create dividend table: %w", err)
	}

	if _, err := conn.Exec(ctx, TradingTable); err != nil {
		return fmt.Errorf("failed to create trading table: %w", err)
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE trading_session ADD COLUMN IF NOT EXISTS symbols text[] NOT NULL DEFAULT '{}'`); err != nil {
		return fmt.Errorf("trading_session symbols: %w", err)
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

	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("failed to create extension btree_gist: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetTable); err != nil {
		return fmt.Errorf("failed to create asset table: %w", err)
	}

	if _, err := conn.Exec(ctx, InstrumentTable); err != nil {
		return fmt.Errorf("failed to create instrument table: %w", err)
	}

	if _, err := conn.Exec(ctx, AssetSymbolTable); err != nil {
		return fmt.Errorf("failed to create asset_symbol table: %w", err)
	}

	if _, err := conn.Exec(ctx, OHLCTable); err != nil {
		return fmt.Errorf("failed to create ohlc table: %w", err)
	}

	if err := migrateFinanceSchema(ctx, conn); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, DividendTable); err != nil {
		return fmt.Errorf("failed to create dividend table: %w", err)
	}

	if _, err := conn.Exec(ctx, TradingTable); err != nil {
		return fmt.Errorf("failed to create trading table: %w", err)
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE trading_session ADD COLUMN IF NOT EXISTS symbols text[] NOT NULL DEFAULT '{}'`); err != nil {
		return fmt.Errorf("trading_session symbols: %w", err)
	}

	if _, err := conn.Exec(ctx, CalendarTable); err != nil {
		return fmt.Errorf("failed to create trading_calendar table: %w", err)
	}

	return nil
}

func migrateFinanceSchema(ctx context.Context, conn *pgxpool.Pool) error {
	if err := MigrateOHLC(ctx, conn); err != nil {
		return err
	}
	if err := MigrateOHLCBars(ctx, conn); err != nil {
		return err
	}
	if err := MigrateAssetSymbol(ctx, conn); err != nil {
		return err
	}
	if err := MigrateProviderSymbol(ctx, conn); err != nil {
		return err
	}
	return BackfillProviderSymbols(ctx, conn)
}
