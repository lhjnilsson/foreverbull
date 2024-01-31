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
backtest text not null,
timestamp TIMESTAMP not null,
shorts_count INTEGER,
pnl INTEGER,
long_value INTEGER,
short_value INTEGER,
long_exposure INTEGER,
starting_exposure INTEGER,
short_exposure INTEGER,
capital_used INTEGER,
gross_leverage INTEGER,
net_leverage INTEGER,
ending_exposure INTEGER,
starting_value INTEGER,
ending_value INTEGER,
starting_cash INTEGER,
ending_cash INTEGER,
returns INTEGER,
portfolio_value INTEGER,
longs_count INTEGER,
algo_volatility INTEGER,
sharpe INTEGER,
alpha INTEGER,
beta INTEGER,
sortino INTEGER,
max_drawdown INTEGER,
max_leverage INTEGER,
excess_returns INTEGER,
treasury_period_return INTEGER,
trading_days INTEGER,
benchmark_period_returns INTEGER,
benchmark_volatility INTEGER,
algorithm_period_return INTEGER);
`

// Ugly and silent way to add constraint. Will always fail if constraint exists
func CreateConstraint(ctx context.Context, db *pgxpool.Pool) {
	db.Exec(ctx, `ALTER TABLE backtest_period ADD CONSTRAINT unique_backtest_period UNIQUE(backtest, timestamp);`)
}

type Period struct {
	Conn postgres.Query
}

/*
List
Returns all results from a backtest
*/
func (db *Period) List(ctx context.Context, backtestID string) (*[]time.Time, error) {
	rows, err := db.Conn.Query(ctx, `select timestamp from backtest_period where backtest=$1 order by timestamp desc`, backtestID)
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
	and column_name not in ('id','backtest','timestamp')`)
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
	rows, err := db.Conn.Query(ctx, fmt.Sprintf(`select %s from backtest_period where backtest=$1 order by timestamp desc`, metric), execution)
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
func (db *Period) Store(ctx context.Context, backtest string, period *entity.Period) error {
	err := db.Conn.QueryRow(ctx,
		`INSERT INTO backtest_period(
		backtest,timestamp,shorts_count,pnl,long_value,short_value,
		long_exposure,starting_exposure,short_exposure,capital_used,
		gross_leverage,net_leverage,ending_exposure,starting_value,
		ending_value,starting_cash,ending_cash,returns,portfolio_value,
		longs_count,algo_volatility,sharpe,alpha,beta,sortino,
		max_drawdown,max_leverage,excess_returns,treasury_period_return,
		trading_days,benchmark_period_returns,benchmark_volatility,algorithm_period_return
	) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,
				$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33) 
		ON CONFLICT (backtest, timestamp) DO UPDATE SET shorts_count=$3,pnl=$4,long_value=$5,short_value=$6,long_exposure=$7,
		starting_exposure=$8,short_exposure=$9,capital_used=$10,gross_leverage=$11,net_leverage=$12,ending_exposure=$13,
		starting_value=$14,ending_value=$15,starting_cash=$16,ending_cash=$17,returns=$18,portfolio_value=$19,longs_count=$20,
		algo_volatility=$21,sharpe=$22,alpha=$23,beta=$24,sortino=$25,max_drawdown=$26,max_leverage=$27,excess_returns=$28,
		treasury_period_return=$29,trading_days=$30,benchmark_period_returns=$31,benchmark_volatility=$32,
		algorithm_period_return=$33  RETURNING id`,
		backtest,
		period.Timestamp,
		period.ShortsCount,
		period.PNL,
		period.LongValue,
		period.ShortValue,
		period.LongExposure,
		period.StartingExposure,
		period.ShortExposure,
		period.CapitalUsed,
		period.GrossLeverage,
		period.NetLeverage,
		period.EndingExposure,
		period.StartingValue,
		period.EndingValue,
		period.StartingCash,
		period.EndingCash,
		period.Returns,
		period.PortfolioValue,
		period.LongsCount,
		period.AlgoVolatility,
		period.Sharpe,
		period.Alpha,
		period.Beta,
		period.Sortino,
		period.MaxDrawdown,
		period.MaxLeverage,
		period.ExcessReturns,
		period.TreasuryPeriodReturn,
		period.TradingDays,
		period.BenchmarkPeriodReturns,
		period.BenchmarkVolatility,
		period.AlgorithmPeriodReturns,
	).Scan(&period.ID)
	if err != nil {
		return fmt.Errorf("error creating period result: %w", err)
	}
	return nil
}
