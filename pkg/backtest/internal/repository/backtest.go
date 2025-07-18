package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	commonPB "github.com/lhjnilsson/foreverbull/pkg/pb"
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
    symbol text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    PRIMARY KEY (backtest_name, start_date, end_date, symbol),
    FOREIGN KEY (backtest_name, start_date, end_date) 
        REFERENCES backtest_period(backtest_name, start_date, end_date) ON DELETE CASCADE
);
`

type Backtest struct {
	Conn postgres.Query
}

func (db *Backtest) Create(ctx context.Context, backtest *backtest.Backtest) (*backtest.Backtest, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO backtest (name, benchmark)
		VALUES ($1, $2)`,
		backtest.Name, backtest.Benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to create backtest: %w", err)
	}
	for _, period := range backtest.Periods {
		_, err := db.Conn.Exec(ctx,
			`INSERT INTO backtest_period (backtest_name, start_date, end_date)
			VALUES ($1, $2, $3)`,
			backtest.Name, commonPB.DateToDateString(period.StartDate), commonPB.DateToDateString(period.EndDate))
		if err != nil {
			return nil, fmt.Errorf("failed to create backtest period: %w", err)
		}
		for _, asset := range period.Assets {
			_, err := db.Conn.Exec(ctx,
				`INSERT INTO backtest_period_asset (backtest_name, start_date, end_date, symbol, enabled)
				VALUES ($1, $2, $3, $4, $5)`,
				backtest.Name, commonPB.DateToDateString(period.StartDate), commonPB.DateToDateString(period.EndDate), asset.Symbol, asset.Enabled)
			if err != nil {
				return nil, fmt.Errorf("failed to create backtest period asset: %w", err)
			}
		}
	}
	return db.Get(ctx, backtest.Name)
}

func (db *Backtest) Get(ctx context.Context, name string) (*backtest.Backtest, error) {
	b := backtest.Backtest{}
	err := db.Conn.QueryRow(ctx,
		`SELECT name, benchmark FROM backtest WHERE name=$1`, name).Scan(&b.Name, &b.Benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest: %w", err)
	}
	rows, err := db.Conn.Query(ctx, `SELECT start_date, end_date FROM backtest_period WHERE backtest_name=$1`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list periods: %w", err)
	}
	periods := make([]*backtest.Backtest_Period, 0)
	defer rows.Close()
	for rows.Next() {
		var startDate, endDate time.Time
		err = rows.Scan(&startDate, &endDate)
		period := backtest.Backtest_Period{
			StartDate: commonPB.GoTimeToDate(startDate),
			EndDate:   commonPB.GoTimeToDate(endDate),
		}
		if err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}
		assetRows, err := db.Conn.Query(ctx, `SELECT symbol, enabled FROM backtest_period_asset WHERE backtest_name=$1 AND start_date=$2 AND end_date=$3`, name, commonPB.DateToDateString(period.StartDate), commonPB.DateToDateString(period.EndDate))
		if err != nil {
			return nil, fmt.Errorf("failed to list period assets: %w", err)
		}
		period.Assets = make([]*backtest.Backtest_Period_Asset, 0)

		for assetRows.Next() {
			var symbol string
			var enabled bool
			err = assetRows.Scan(&symbol, &enabled)
			if err != nil {
				return nil, fmt.Errorf("failed to scan asset: %w", err)
			}
			period.Assets = append(period.Assets, &backtest.Backtest_Period_Asset{
				Symbol:  symbol,
				Enabled: enabled,
			})
		}
		periods = append(periods, &period)
	}
	b.Periods = periods
	return &b, nil
}

func (db *Backtest) UpdateBenchmark(ctx context.Context, name string, benchmark *string,
) (*backtest.Backtest, error) {
	_, err := db.Conn.Exec(ctx, `UPDATE backtest SET benchmark=$2 WHERE name=$1`, name, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to update backtest: %w", err)
	}
	return db.Get(ctx, name)
}

func (db *Backtest) UpdatePeriod(ctx context.Context, name string, period *backtest.Backtest_Period) error {
	for _, asset := range period.Assets {
		_, err := db.Conn.Exec(ctx, `INSERT INTO backtest_period_asset (backtest_name, start_date, end_date, symbol, enabled)
		VALUES ($1, $2, $3, $4, $5)`,
			name, commonPB.DateToDateString(period.StartDate), commonPB.DateToDateString(period.EndDate), asset.Symbol, asset.Enabled)
		if err != nil {
			return fmt.Errorf("failed to update backtest period asset: %w", err)
		}
	}
	return nil
}

func (db *Backtest) UpdatePeriodAsset(ctx context.Context, name string, period *backtest.Backtest_Period, asset *backtest.Backtest_Period_Asset) error {
	_, err := db.Conn.Exec(ctx, `UPDATE backtest_period_asset SET enabled=$5 WHERE backtest_name=$1 AND start_date=$2 AND end_date=$3 AND symbol=$4`,
		name, commonPB.DateToDateString(period.StartDate), commonPB.DateToDateString(period.EndDate), asset.Symbol, asset.Enabled)
	if err != nil {
		return fmt.Errorf("failed to update backtest period asset: %w", err)
	}
	return nil
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
