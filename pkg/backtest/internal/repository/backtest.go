package repository

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

const BacktestTable = `
CREATE TABLE IF NOT EXISTS backtest (
	name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
	benchmark text);

CREATE TABLE IF NOT EXISTS backtest_period (
	backtest_name text REFERENCES backtest (name) ON DELETE CASCADE,
	start_date date NOT NULL,
	end_date date NOT NULL,
	PRIMARY KEY (backtest_name, start_date, end_date)
);

CREATE TABLE IF NOT EXISTS backtest_period_asset (
    backtest_name text,
    start_date date,
    end_date date,
    asset_symbol text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    PRIMARY KEY (backtest_name, start_date, end_date, asset_symbol),
    FOREIGN KEY (backtest_name, start_date, end_date) 
        REFERENCES backtest_period(backtest_name, start_date, end_date) ON DELETE CASCADE
);
`

type Backtest struct {
	Conn postgres.Query
}

func (db *Backtest) Create(ctx context.Context, name string, benchmark *string) (*backtest.Backtest, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO backtest (name, benchmark)
		VALUES ($1, $2)`,
		name, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to create backtest: %w", err)
	}

	return db.Get(ctx, name)
}

func (db *Backtest) Get(ctx context.Context, name string) (*backtest.Backtest, error) {
	b := backtest.Backtest{}
	err := db.Conn.QueryRow(ctx,
		`SELECT name, benchmark FROM backtest WHERE name=$1`, name).Scan(&b.Name, &b.Benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest: %w", err)
	}
	return &b, nil
}

func (db *Backtest) Update(ctx context.Context, name string, benchmark *string,
) (*backtest.Backtest, error) {
	_, err := db.Conn.Exec(ctx, `UPDATE backtest SET benchmark=$2 WHERE name=$1`, name, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to update backtest: %w", err)
	}
	return db.Get(ctx, name)
}

func (db *Backtest) List(ctx context.Context) ([]*backtest.Backtest, error) {
	rows, err := db.Conn.Query(ctx, `SELECT name, benchmark FROM backtest`)
	if err != nil {
		return nil, fmt.Errorf("failed to list backtests: %w", err)
	}
	defer rows.Close()

	backtests := make([]*backtest.Backtest, 0)

	for rows.Next() {
		backtest := backtest.Backtest{}

		err = rows.Scan(&backtest.Name, &backtest.Benchmark)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backtest: %w", err)
		}
		backtests = append(backtests, &backtest)
	}

	return backtests, nil
}

func (db *Backtest) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx, `DELETE FROM backtest WHERE name=$1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete backtest: %w", err)
	}

	return nil
}
