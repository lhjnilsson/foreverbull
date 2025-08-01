package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	commonPB "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

const BacktestTable = `
CREATE TABLE IF NOT EXISTS backtest (
	name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
	benchmark text NOT NULL
);

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
	type rowData struct {
		Name      string
		Benchmark string
		StartDate *time.Time
		EndDate   *time.Time
		Symbol    *string
		Enabled   *bool
	}
	rows, err := db.Conn.Query(ctx, `
	    SELECT 
			backtest.name,
			backtest.benchmark,
			backtest_period.start_date,
			backtest_period.end_date,
			backtest_period_asset.symbol,
			backtest_period_asset.enabled
		FROM backtest
		LEFT JOIN backtest_period ON backtest.name = backtest_period.backtest_name
		LEFT JOIN backtest_period_asset ON backtest_period.backtest_name = backtest_period_asset.backtest_name
		AND backtest_period.start_date = backtest_period_asset.start_date
		WHERE backtest.name = $1
		ORDER BY backtest_period.start_date, backtest_period_asset.symbol DESC
	`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest: %w", err)
	}
	defer rows.Close()

	b := backtest.Backtest{}
	periodsMap := make(map[string]*backtest.Backtest_Period)

	for rows.Next() {
		var row rowData
		err = rows.Scan(&row.Name, &row.Benchmark, &row.StartDate, &row.EndDate, &row.Symbol, &row.Enabled)
		if err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}

		b.Name = row.Name
		b.Benchmark = row.Benchmark

		if row.StartDate == nil || row.EndDate == nil {
			continue
		}

		period, ok := periodsMap[row.StartDate.String()+"_"+row.EndDate.String()]
		if !ok {
			period = &backtest.Backtest_Period{
				StartDate: commonPB.GoTimeToDate(*row.StartDate),
				EndDate:   commonPB.GoTimeToDate(*row.EndDate),
				Assets:    make([]*backtest.Backtest_Period_Asset, 0),
			}
			periodsMap[row.StartDate.String()+"_"+row.EndDate.String()] = period
		}
		if row.Symbol == nil {
			continue
		}
		period.Assets = append(period.Assets, &backtest.Backtest_Period_Asset{
			Symbol:  *row.Symbol,
			Enabled: *row.Enabled,
		})
	}
	for _, period := range periodsMap {
		b.Periods = append(b.Periods, period)
	}
	if b.Name == "" {
		return nil, pgx.ErrNoRows
	}
	return &b, nil
}

func (db *Backtest) UpdateBenchmark(ctx context.Context, name string, benchmark string,
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

func (db *Backtest) UpdatePeriodAsset(ctx context.Context, name string, asset_symbol string, enabled bool) error {
	_, err := db.Conn.Exec(ctx, `UPDATE backtest_period_asset SET enabled=$3 WHERE backtest_name=$1 AND symbol=$2`,
		name, asset_symbol, enabled)
	if err != nil {
		return fmt.Errorf("failed to update backtest period asset: %w", err)
	}
	return nil
}

func (db *Backtest) List(ctx context.Context) ([]*backtest.Backtest, error) {
	type rowData struct {
		Name      string
		Benchmark string
		StartDate *time.Time
		EndDate   *time.Time
		Symbol    *string
		Enabled   *bool
	}
	rows, err := db.Conn.Query(ctx, `
	    SELECT 
			backtest.name,
			backtest.benchmark,
			backtest_period.start_date,
			backtest_period.end_date,
			backtest_period_asset.symbol,
			backtest_period_asset.enabled
		FROM backtest
		LEFT JOIN backtest_period ON backtest.name = backtest_period.backtest_name
		LEFT JOIN backtest_period_asset ON backtest_period.backtest_name = backtest_period_asset.backtest_name
		AND backtest_period.start_date = backtest_period_asset.start_date
		ORDER BY backtest_period.start_date, backtest_period_asset.symbol DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list backtests: %w", err)
	}
	defer rows.Close()

	type backtestPeriodMap struct {
		backtest *backtest.Backtest
		periods  map[string]*backtest.Backtest_Period
	}
	backtestMap := make(map[string]backtestPeriodMap)

	for rows.Next() {
		var row rowData
		err = rows.Scan(&row.Name, &row.Benchmark, &row.StartDate, &row.EndDate, &row.Symbol, &row.Enabled)
		if err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}
		b, ok := backtestMap[row.Name]
		if !ok {
			b = backtestPeriodMap{
				backtest: &backtest.Backtest{
					Name:      row.Name,
					Benchmark: row.Benchmark,
				},
				periods: make(map[string]*backtest.Backtest_Period),
			}
			backtestMap[row.Name] = b
		}
		if row.StartDate == nil || row.EndDate == nil {
			continue
		}
		period, ok := b.periods[row.StartDate.String()+"_"+row.EndDate.String()]
		if !ok {
			period = &backtest.Backtest_Period{
				StartDate: commonPB.GoTimeToDate(*row.StartDate),
				EndDate:   commonPB.GoTimeToDate(*row.EndDate),
				Assets:    make([]*backtest.Backtest_Period_Asset, 0),
			}
			b.periods[row.StartDate.String()+"_"+row.EndDate.String()] = period
		}
		if row.Symbol == nil {
			continue
		}
		period.Assets = append(period.Assets, &backtest.Backtest_Period_Asset{
			Symbol:  *row.Symbol,
			Enabled: *row.Enabled,
		})
	}
	backtests := make([]*backtest.Backtest, 0)
	for _, b := range backtestMap {
		for _, period := range b.periods {
			b.backtest.Periods = append(b.backtest.Periods, period)
		}
		backtests = append(backtests, b.backtest)
	}
	return backtests, nil
}

func (db *Backtest) Delete(ctx context.Context, name string) error {
	result, err := db.Conn.Exec(ctx, `DELETE FROM backtest WHERE name=$1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete backtest: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
