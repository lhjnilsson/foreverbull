package repository

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
)

const StrategyTable = `CREATE TABLE IF NOT EXISTS strategy (
name text PRIMARY KEY,
backtest text,
schedule text,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`

type Strategy struct {
	Conn postgres.Query
}

func (db *Strategy) Create(ctx context.Context, strategy *entity.Strategy) error {
	return db.Conn.QueryRow(ctx,
		"INSERT INTO strategy (name, backtest, schedule) VALUES ($1, $2, $3) RETURNING created_at",
		strategy.Name, strategy.Backtest, strategy.Schedule).Scan(
		&strategy.CreatedAt)
}

func (db *Strategy) List(ctx context.Context) (*[]entity.Strategy, error) {
	rows, err := db.Conn.Query(ctx, "SELECT name, backtest, created_at FROM strategy")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	strategies := make([]entity.Strategy, 0)
	for rows.Next() {
		s := entity.Strategy{}
		err := rows.Scan(&s.Name, &s.Backtest, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		strategies = append(strategies, s)
	}
	return &strategies, nil
}

func (db *Strategy) Get(ctx context.Context, name string) (*entity.Strategy, error) {
	s := entity.Strategy{}
	err := db.Conn.QueryRow(ctx,
		"SELECT name, backtest, schedule, created_at FROM strategy WHERE name=$1", name).Scan(
		&s.Name, &s.Backtest, &s.Schedule, &s.CreatedAt)
	return &s, err
}

func (db *Strategy) SetBacktest(ctx context.Context, name string, backtest string) error {
	_, err := db.Conn.Exec(ctx, "UPDATE strategy SET backtest=$1 WHERE name=$2",
		backtest, name)
	return err
}

func (db *Strategy) SetSchedule(ctx context.Context, name string, schedule string) error {
	_, err := db.Conn.Exec(ctx, "UPDATE strategy SET schedule=$1 WHERE name=$2",
		schedule, name)
	return err
}

func (db *Strategy) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx, "DELETE FROM strategy WHERE name=$1", name)
	return err
}
