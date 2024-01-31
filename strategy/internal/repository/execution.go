package repository

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
)

const ExecutionTable = `
CREATE TABLE IF NOT EXISTS strategy_execution (
id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
strategy text NOT NULL, 
started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
ingested_at TIMESTAMPTZ,
backtest_at TIMESTAMPTZ,
backtest_session_id text);`

type Execution struct {
	Conn postgres.Query
}

func (db *Execution) Create(ctx context.Context, strategy string) (*entity.Execution, error) {
	w := entity.Execution{Strategy: &strategy}
	err := db.Conn.QueryRow(ctx,
		`INSERT INTO strategy_execution (strategy) VALUES ($1) 
		RETURNING id, started_at, ingested_at, backtest_at`, strategy).Scan(
		&w.ID, &w.StartedAt, &w.IngestedAt, &w.BacktestAt)
	return &w, err
}

func (db *Execution) List(ctx context.Context, strategy string) (*[]entity.Execution, error) {
	rows, err := db.Conn.Query(ctx, "SELECT id, started_at, ingested_at, backtest_at FROM strategy_execution WHERE strategy=$1", strategy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := make([]entity.Execution, 0)
	for rows.Next() {
		w := entity.Execution{Strategy: &strategy}
		err := rows.Scan(&w.ID, &w.StartedAt, &w.IngestedAt, &w.BacktestAt)
		if err != nil {
			return nil, err
		}
		executions = append(executions, w)
	}
	return &executions, nil
}

func (db *Execution) Get(ctx context.Context, id string) (*entity.Execution, error) {
	w := entity.Execution{ID: &id}
	err := db.Conn.QueryRow(ctx,
		"SELECT strategy, started_at, ingested_at, backtest_at FROM strategy_execution WHERE id=$1", id).Scan(
		&w.Strategy, &w.StartedAt, &w.IngestedAt, &w.BacktestAt)
	return &w, err
}

func (db *Execution) GetByBacktestAndBacktestAtisNull(ctx context.Context, backtestName string) (*[]entity.Execution, error) {
	rows, err := db.Conn.Query(ctx, `SELECT id, strategy, started_at, ingested_at, backtest_at, backtest_session_id 
	FROM strategy_execution INNER JOIN strategy ON strategy_execution.strategy=strategy.name
	WHERE strategy.backtest=$1 AND backtest_at IS NULL`, backtestName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := make([]entity.Execution, 0)
	for rows.Next() {
		w := entity.Execution{}
		err := rows.Scan(&w.ID, &w.Strategy, &w.StartedAt, &w.IngestedAt, &w.BacktestAt, &w.BacktestSessionID)
		if err != nil {
			return nil, err
		}
		executions = append(executions, w)
	}
	return &executions, nil
}

func (db *Execution) SetIngestedAt(ctx context.Context, id string) error {
	_, err := db.Conn.Exec(ctx, "UPDATE strategy_execution SET ingested_at=NOW() WHERE id=$1", id)
	return err
}

func (db *Execution) SetBacktestAt(ctx context.Context, id string, backtestSessionID string) error {
	_, err := db.Conn.Exec(ctx, "UPDATE strategy_execution SET backtest_at=NOW() WHERE id=$1", id)
	return err
}
