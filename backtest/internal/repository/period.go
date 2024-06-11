package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const PeriodTable = `CREATE TABLE IF NOT EXISTS backtest_period (
id serial primary key,
backtest_execution text not null,

timestamp timestamp not null,
pnl numeric  not null,
returns numeric  not null,
portfolio_value numeric  not null,

longs_count integer,
shorts_count integer,
long_value numeric  not null,
short_value numeric  not null,
starting_exposure numeric  not null,
ending_exposure numeric  not null,
long_exposure numeric  not null,
short_exposure numeric  not null,

capital_used numeric  not null,
gross_leverage numeric  not null,
net_leverage numeric  not null,

starting_value numeric  not null,
ending_value numeric  not null,
starting_cash numeric  not null,
ending_cash numeric  not null,

max_drawdown numeric  not null,
max_leverage numeric  not null,
excess_returns numeric  not null,
treasury_period_return numeric  not null,
algorithm_period_return numeric  not null,

algo_volatility numeric,
sharpe numeric,
sortino numeric,

benchmark_period_returns numeric,
benchmark_volatility numeric,
alpha numeric,
beta numeric);
`

// Ugly and silent way to add constraint. Will always fail if constraint exists
func CreateConstraint(ctx context.Context, db *pgxpool.Pool) {
	_, err := db.Exec(ctx, `ALTER TABLE backtest_period ADD CONSTRAINT unique_backtest_period UNIQUE(backtest_execution, timestamp);`)
	if err != nil {
		fmt.Println("err creating constraint: ", err)
	}
}

type Period struct {
	Conn postgres.Query
}

/*
List
Returns all results from a backtest
*/
func (db *Period) List(ctx context.Context, backtestID string) (*[]time.Time, error) {
	rows, err := db.Conn.Query(ctx, `select timestamp from backtest_period where backtest_execution=$1 order by timestamp desc`, backtestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	timestamps := make([]time.Time, 0)
	for rows.Next() {
		var timestamp time.Time
		err := rows.Scan(&timestamp)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, timestamp)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return &timestamps, nil
}

func (db *Period) Metrics(ctx context.Context, execution string) (*[]string, error) {
	rows, err := db.Conn.Query(ctx, `select column_name from information_schema.columns where table_name='backtest_period'
	and column_name not in ('id','backtest_execution','timestamp')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	metrics := make([]string, 0)
	for rows.Next() {
		var metric string
		err := rows.Scan(&metric)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return &metrics, nil
}

func (db *Period) Metric(ctx context.Context, execution string, metric string) (*[]int, error) {
	rows, err := db.Conn.Query(ctx, fmt.Sprintf(`select %s from backtest_period where backtest_execution=$1 order by timestamp desc`, metric), execution)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var metrics []int
	for rows.Next() {
		var m int
		err := rows.Scan(&m)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return &metrics, nil
}

/*
Create
Adds a new entry to the result of a backtest
*/
func (db *Period) Store(ctx context.Context, execution string, period *entity.Period) error {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO backtest_period(
			backtest_execution, timestamp, pnl, returns, portfolio_value, 
			longs_count, shorts_count, long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure, 
			capital_used, gross_leverage, net_leverage, 
			starting_value, ending_value, starting_cash, ending_cash, 
			max_drawdown, max_leverage, excess_returns, treasury_period_return, algorithm_period_return, 
			algo_volatility, sharpe, sortino, 
			benchmark_period_returns, benchmark_volatility, alpha, beta)
		VALUES($1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23, $24, $25,
			$26, $27, $28,
			$29, $30, $31, $32)`,
		execution, period.Timestamp, period.PNL, period.Returns, period.PortfolioValue,
		period.LongsCount, period.ShortsCount, period.LongValue, period.ShortValue, period.StartingExposure, period.EndingExposure, period.LongExposure, period.ShortExposure,
		period.CapitalUsed, period.GrossLeverage, period.NetLeverage,
		period.StartingValue, period.EndingValue, period.StartingCash, period.EndingCash,
		period.MaxDrawdown, period.MaxLeverage, period.ExcessReturns, period.TreasuryPeriodReturn, period.AlgorithmPeriodReturns,
		period.AlgoVolatility, period.Sharpe, period.Sortino,
		period.BenchmarkPeriodReturns, period.BenchmarkVolatility, period.Alpha, period.Beta,
	)
	if err != nil {
		return fmt.Errorf("error creating period result: %w", err)
	}
	return nil
}
