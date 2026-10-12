package repository_test

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"google.golang.org/protobuf/types/known/structpb"
)

func insertService(ctx context.Context, conn *pgxpool.Pool, image string) error {
	_, err := conn.Exec(ctx, `INSERT INTO service (image) VALUES ($1)`, image)
	return err
}

func insertPeriod(
	ctx context.Context,
	conn *pgxpool.Pool,
	executionID, date string,
	portfolioValue, maxDrawdown, algorithmPeriodReturn float64,
	sharpe, benchmark *float64,
) error {
	_, err := conn.Exec(ctx, `
		INSERT INTO backtest_execution_period (
			backtest_execution, date, pnl, returns, portfolio_value,
			long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure,
			capital_used, gross_leverage, net_leverage,
			starting_value, ending_value, starting_cash, ending_cash,
			max_drawdown, max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
			sharpe, benchmark_period_return
		) VALUES (
			$1, $2, 0, 0, $3,
			0, 0, 0, 0, 0, 0,
			0, 0, 0,
			0, 0, 0, 0,
			$4, 0, 0, 0, $5,
			$6, $7
		)`,
		executionID, date, portfolioValue, maxDrawdown, algorithmPeriodReturn, sharpe, benchmark,
	)
	return err
}

func floatPtr(v float64) *float64 {
	return &v
}

func insertBacktest(ctx context.Context, conn *pgxpool.Pool, names ...string) error {
	for _, name := range names {
		_, err := conn.Exec(ctx,
			`INSERT INTO backtest (name, benchmark) VALUES ($1, 'SPY') ON CONFLICT (name) DO NOTHING`,
			name)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertExecution(ctx context.Context, conn *pgxpool.Pool, backtestName string) (string, error) {
	_, err := conn.Exec(ctx,
		`INSERT INTO backtest (name, benchmark) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`,
		backtestName, "SPY")
	if err != nil {
		return "", err
	}

	var sessionID string
	err = conn.QueryRow(ctx,
		`INSERT INTO session (backtest) VALUES ($1) RETURNING id`,
		backtestName,
	).Scan(&sessionID)
	if err != nil {
		return "", err
	}

	var executionID string
	err = conn.QueryRow(ctx,
		`INSERT INTO backtest_execution (session, start_date, end_date)
		VALUES ($1, $2, $3) RETURNING id`,
		sessionID, "2020-01-01", "2020-12-31",
	).Scan(&executionID)
	if err != nil {
		return "", err
	}

	return executionID, nil
}

func createStrategyAndVersion(
	ctx context.Context,
	conn *pgxpool.Pool,
	name, image string,
	parameters *structpb.Struct,
) (*pb.Strategy, *pb.Version, error) {
	if err := insertService(ctx, conn, image); err != nil {
		return nil, nil, err
	}

	strategies := repository.Strategy{Conn: conn}
	strategy, err := strategies.Create(ctx, name, nil)
	if err != nil {
		return nil, nil, err
	}

	versions := repository.Version{Conn: conn}
	version, err := versions.Create(ctx, name, image, parameters)
	if err != nil {
		return nil, nil, err
	}

	return strategy, version, nil
}
