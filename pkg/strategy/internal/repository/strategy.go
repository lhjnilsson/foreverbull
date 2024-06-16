package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/entity"
)

const StrategyTable = `CREATE TABLE IF NOT EXISTS strategy (
name text PRIMARY KEY,
symbols text[] NOT NULL,
min_days int NOT NULL,
service text NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`

type Strategy struct {
	Conn postgres.Query
}

func (db *Strategy) Create(ctx context.Context, name string, symbols []string, min_days int, service string) (*entity.Strategy, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO strategy (name, symbols, min_days, service) VALUES ($1, $2, $3, $4)`,
		name, symbols, min_days, service)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, name)
}

func (db *Strategy) List(ctx context.Context) (*[]entity.Strategy, error) {
	rows, err := db.Conn.Query(ctx, "SELECT name, symbols, min_days, service, created_at FROM strategy")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	strategies := []entity.Strategy{}
	for rows.Next() {
		s := entity.Strategy{}
		err = rows.Scan(&s.Name, &s.Symbols, &s.MinDays, &s.Service, &s.CreatedAt)
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
		"SELECT name, symbols, min_days, service, created_at FROM strategy WHERE name=$1",
		name).Scan(&s.Name, &s.Symbols, &s.MinDays, &s.Service, &s.CreatedAt)
	if s.Name == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &s, err
}

func (db *Strategy) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx, "DELETE FROM strategy WHERE name=$1", name)
	return err
}
