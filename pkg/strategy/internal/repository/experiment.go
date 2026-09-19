package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
)

const ExperimentTable = `
CREATE TABLE IF NOT EXISTS strategy_experiment (
	id text PRIMARY KEY DEFAULT uuid_generate_v4(),
	strategy_id text NOT NULL REFERENCES strategy(id) ON DELETE CASCADE,
	version_id text NOT NULL REFERENCES strategy_version(id) ON DELETE CASCADE,
	backtest_name text NOT NULL,
	execution_id text REFERENCES backtest_execution(id),
	status int NOT NULL DEFAULT 0,
	error text,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	completed_at TIMESTAMPTZ
);
`

type Experiment struct {
	Conn postgres.Query
}

func (db *Experiment) Create(ctx context.Context, versionID, backtestName string) (*pb.Experiment, error) {
	var id string
	err := db.Conn.QueryRow(ctx,
		`INSERT INTO strategy_experiment (strategy_id, version_id, backtest_name, status)
		SELECT strategy_id, id, $2, $3 FROM strategy_version WHERE id=$1
		RETURNING id`,
		versionID, backtestName, pb.Experiment_QUEUED,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create strategy experiment: %w", err)
	}

	return db.Get(ctx, id)
}

func (db *Experiment) Get(ctx context.Context, id string) (*pb.Experiment, error) {
	e, err := db.scanOne(ctx, experimentSelect+` WHERE e.id=$1`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy experiment: %w", err)
	}

	return e, nil
}

func (db *Experiment) List(ctx context.Context, strategyName, versionID, backtestName string) ([]*pb.Experiment, error) {
	rows, err := db.Conn.Query(ctx, experimentSelect+`
		WHERE ($1 = '' OR s.name = $1)
		AND ($2 = '' OR e.version_id = $2)
		AND ($3 = '' OR e.backtest_name = $3)
		ORDER BY e.created_at ASC`,
		strategyName, versionID, backtestName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy experiments: %w", err)
	}
	defer rows.Close()

	return parseExperimentRows(rows)
}

func (db *Experiment) UpdateStatus(ctx context.Context, id string, status pb.Experiment_Status, statusErr error) error {
	var errText *string
	if statusErr != nil {
		msg := statusErr.Error()
		errText = &msg
	}

	query := `UPDATE strategy_experiment SET status=$2, error=$3 WHERE id=$1`
	if status == pb.Experiment_COMPLETED || status == pb.Experiment_FAILED {
		query = `UPDATE strategy_experiment SET status=$2, error=$3, completed_at=NOW() WHERE id=$1`
	}

	_, err := db.Conn.Exec(ctx, query, id, status, errText)
	if err != nil {
		return fmt.Errorf("failed to update experiment status: %w", err)
	}

	return nil
}

func (db *Experiment) SetExecutionID(ctx context.Context, id, executionID string) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE strategy_experiment SET execution_id=$2 WHERE id=$1`,
		id, executionID,
	)
	if err != nil {
		return fmt.Errorf("failed to set experiment execution id: %w", err)
	}

	return nil
}

func (db *Experiment) GetLastPeriod(ctx context.Context, executionID string) (*backtest.Period, error) {
	if executionID == "" {
		return nil, nil
	}

	var (
		periodDate            time.Time
		portfolioValue        float64
		maxDrawdown           float64
		sharpe                *float64
		algorithmPeriodReturn float64
		benchmarkPeriodReturn *float64
	)

	err := db.Conn.QueryRow(ctx, `
		SELECT date, portfolio_value, max_drawdown, sharpe, algorithm_period_return, benchmark_period_return
		FROM backtest_execution_period
		WHERE backtest_execution=$1
		ORDER BY date DESC
		LIMIT 1`, executionID,
	).Scan(&periodDate, &portfolioValue, &maxDrawdown, &sharpe, &algorithmPeriodReturn, &benchmarkPeriodReturn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last execution period: %w", err)
	}

	return &backtest.Period{
		Date:                  internal_pb.GoTimeToDate(periodDate),
		PortfolioValue:        portfolioValue,
		MaxDrawdown:           maxDrawdown,
		Sharpe:                sharpe,
		AlgorithmPeriodReturn: algorithmPeriodReturn,
		BenchmarkPeriodReturn: benchmarkPeriodReturn,
	}, nil
}

const experimentSelect = `
SELECT e.id, e.strategy_id, s.name, e.version_id, v.rev, e.backtest_name,
	e.execution_id, e.status, e.error, e.created_at, e.completed_at,
	p.portfolio_value, p.max_drawdown, p.sharpe, p.algorithm_period_return, p.benchmark_period_return
FROM strategy_experiment e
JOIN strategy s ON e.strategy_id = s.id
JOIN strategy_version v ON e.version_id = v.id
LEFT JOIN LATERAL (
	SELECT portfolio_value, max_drawdown, sharpe, algorithm_period_return, benchmark_period_return
	FROM backtest_execution_period
	WHERE backtest_execution = e.execution_id
	ORDER BY date DESC
	LIMIT 1
) p ON true`

func (db *Experiment) scanOne(ctx context.Context, query string, args ...interface{}) (*pb.Experiment, error) {
	e := pb.Experiment{}
	var executionID *string
	var errText *string
	var createdAt time.Time
	var completedAt *time.Time
	var portfolioValue *float64
	var maxDrawdown *float64
	var sharpe *float64
	var algorithmPeriodReturn *float64
	var benchmarkPeriodReturn *float64

	err := db.Conn.QueryRow(ctx, query, args...).Scan(
		&e.Id, &e.StrategyId, &e.StrategyName, &e.VersionId, &e.VersionRev, &e.BacktestName,
		&executionID, &e.Status, &errText, &createdAt, &completedAt,
		&portfolioValue, &maxDrawdown, &sharpe, &algorithmPeriodReturn, &benchmarkPeriodReturn,
	)
	if err != nil {
		return nil, err
	}

	fillExperiment(&e, executionID, errText, createdAt, completedAt,
		experimentStats(portfolioValue, maxDrawdown, sharpe, algorithmPeriodReturn, benchmarkPeriodReturn))

	return &e, nil
}

func parseExperimentRows(rows pgx.Rows) ([]*pb.Experiment, error) {
	experiments := []*pb.Experiment{}

	for rows.Next() {
		e := pb.Experiment{}
		var executionID *string
		var errText *string
		var createdAt time.Time
		var completedAt *time.Time
		var portfolioValue *float64
		var maxDrawdown *float64
		var sharpe *float64
		var algorithmPeriodReturn *float64
		var benchmarkPeriodReturn *float64

		err := rows.Scan(
			&e.Id, &e.StrategyId, &e.StrategyName, &e.VersionId, &e.VersionRev, &e.BacktestName,
			&executionID, &e.Status, &errText, &createdAt, &completedAt,
			&portfolioValue, &maxDrawdown, &sharpe, &algorithmPeriodReturn, &benchmarkPeriodReturn,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan strategy experiment: %w", err)
		}

		fillExperiment(&e, executionID, errText, createdAt, completedAt,
			experimentStats(portfolioValue, maxDrawdown, sharpe, algorithmPeriodReturn, benchmarkPeriodReturn))
		experiments = append(experiments, &e)
	}

	return experiments, nil
}

func fillExperiment(e *pb.Experiment, executionID, errText *string, createdAt time.Time, completedAt *time.Time, stats *pb.ExperimentStats) {
	e.ExecutionId = executionID
	e.Error = errText
	e.CreatedAt = internal_pb.TimeToProtoTimestamp(createdAt)
	if completedAt != nil {
		e.CompletedAt = internal_pb.TimeToProtoTimestamp(*completedAt)
	}
	e.Stats = stats
}

func experimentStats(portfolioValue, maxDrawdown, sharpe, algorithmPeriodReturn, benchmarkPeriodReturn *float64) *pb.ExperimentStats {
	if portfolioValue == nil && maxDrawdown == nil && algorithmPeriodReturn == nil {
		return nil
	}

	stats := &pb.ExperimentStats{
		Sharpe:                sharpe,
		BenchmarkPeriodReturn: benchmarkPeriodReturn,
	}
	if portfolioValue != nil {
		stats.PortfolioValue = *portfolioValue
	}
	if maxDrawdown != nil {
		stats.MaxDrawdown = *maxDrawdown
	}
	if algorithmPeriodReturn != nil {
		stats.AlgorithmPeriodReturn = *algorithmPeriodReturn
	}

	return stats
}
