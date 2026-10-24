package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

// getJoined reads an execution after session.backtest has been replaced by backtest_id.
func (db *Execution) getJoined(ctx context.Context, executionID string) (*backtest.Execution, error) {
	execution := backtest.Execution{}
	start := time.Time{}
	end := pgtype.Date{}

	err := db.Conn.QueryRow(ctx, `
		SELECT backtest_execution.id, backtest.name, backtest_execution.session, start_date, end_date
		FROM backtest_execution
		INNER JOIN session ON backtest_execution.session = session.id
		INNER JOIN backtest ON session.backtest_id = backtest.id
		WHERE backtest_execution.id = $1`, executionID).Scan(
		&execution.Id, &execution.Backtest, &execution.Session, &start, &end,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	execution.StartDate = internal_pb.GoTimeToDate(start)
	if end.Valid {
		execution.EndDate = internal_pb.GoTimeToDate(end.Time)
	}
	return &execution, nil
}

// BacktestID is the backtest that owns this execution. It uses backtest_id.
func (db *Execution) BacktestID(ctx context.Context, executionID string) (string, error) {
	var id string
	err := db.Conn.QueryRow(ctx, `
		SELECT session.backtest_id
		FROM backtest_execution
		INNER JOIN session ON backtest_execution.session = session.id
		WHERE backtest_execution.id = $1`, executionID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (db *Execution) listJoined(ctx context.Context, where string, args []any) ([]*backtest.Execution, error) {
	query := `
		SELECT backtest_execution.id, session.id, backtest.name, backtest_execution.start_date, backtest_execution.end_date,
		ep.date, ep.pnl, ep.returns, ep.portfolio_value, ep.longs_count, ep.shorts_count,
		ep.long_value, ep.short_value, ep.starting_exposure, ep.ending_exposure, ep.long_exposure, ep.short_exposure,
		ep.capital_used, ep.gross_leverage, ep.net_leverage,
		ep.starting_value, ep.ending_value, ep.starting_cash, ep.ending_cash,
		ep.max_drawdown, ep.max_leverage, ep.excess_returns, ep.treasury_period_return, ep.algorithm_period_return,
		ep.algo_volatility, ep.sharpe, ep.sortino,
		ep.benchmark_period_return, ep.benchmark_volatility, ep.alpha, ep.beta
		FROM backtest_execution
		INNER JOIN session ON backtest_execution.session = session.id
		INNER JOIN backtest ON session.backtest_id = backtest.id
		LEFT JOIN (
			SELECT backtest_execution, date, pnl, returns, portfolio_value, longs_count, shorts_count,
			long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure, capital_used, gross_leverage,
			net_leverage, starting_value, ending_value, starting_cash, ending_cash, max_drawdown,
			max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
			algo_volatility, sharpe, sortino, benchmark_period_return, benchmark_volatility,
			alpha, beta FROM backtest_execution_period ORDER BY date DESC LIMIT 1
		) as ep ON backtest_execution.id = ep.backtest_execution ` + where
	rows, err := db.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()
	return db.parseRows(rows)
}

// ListVisible lists executions on backtests the caller may read.
func (db *Execution) ListVisible(ctx context.Context, caller auth.Account) ([]*backtest.Execution, error) {
	if caller.ID == "" {
		return nil, errors.New("account id is empty")
	}
	where, args := visibleWhere("backtest", caller, auth.DomainBacktest)
	return db.listJoined(ctx, "WHERE "+where, args)
}

// ListByBacktestID lists executions for one backtest id.
func (db *Execution) ListByBacktestID(ctx context.Context, backtestID string) ([]*backtest.Execution, error) {
	if backtestID == "" {
		return nil, fmt.Errorf("backtest id is empty")
	}
	return db.listJoined(ctx, "WHERE backtest.id = $1", []any{backtestID})
}
