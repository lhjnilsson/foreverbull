package repository

import (
	"context"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const PortfolioTable = `CREATE TABLE IF NOT EXISTS backtest_portfolio (
id serial primary key,
execution TEXT,
date TIMESTAMP,
cash numeric,
value numeric,
CONSTRAINT unique_portfolio UNIQUE(execution, date));
`

const PositionTable = `CREATE TABLE IF NOT EXISTS backtest_position (
id serial primary key,
portfolio_id INTEGER REFERENCES backtest_portfolio(id),
symbol TEXT,
amount INTEGER,
cost_basis numeric,
period timestamp,
CONSTRAINT unique_position UNIQUE(portfolio_id, symbol, period));
`

type Portfolio struct {
	Conn postgres.Query
}

func (db *Portfolio) Store(ctx context.Context, execution string, date time.Time, portfolio *finance.Portfolio) error {
	var portfolioID int
	err := db.Conn.QueryRow(ctx,
		`INSERT INTO backtest_portfolio (execution, date, cash, value) VALUES ($1, $2, $3, $4) RETURNING id`,
		execution, date, portfolio.Cash, portfolio.Value).Scan(&portfolioID)
	if err != nil {
		return err
	}

	for _, position := range portfolio.Positions {
		_, err = db.Conn.Exec(ctx,
			`INSERT INTO backtest_position (portfolio_id, symbol, amount, cost_basis, period) 
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING;`,
			portfolioID, position.Symbol, position.Amount, position.CostBasis, position.Period)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *Portfolio) GetLatest(ctx context.Context, execution string) (*finance.Portfolio, error) {
	var portfolio finance.Portfolio
	var positions []finance.Position
	var portfolioID int
	err := db.Conn.QueryRow(ctx,
		`SELECT id, cash, value FROM backtest_portfolio WHERE execution = $1 ORDER BY date DESC LIMIT 1`,
		execution).Scan(&portfolioID, &portfolio.Cash, &portfolio.Value)
	if err != nil {
		return nil, err
	}

	rows, err := db.Conn.Query(ctx,
		`SELECT symbol, amount, cost_basis, period FROM backtest_position WHERE portfolio_id = $1`,
		portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var position finance.Position
		err := rows.Scan(&position.Symbol, &position.Amount, &position.CostBasis, &position.Period)
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	portfolio.Positions = positions
	return &portfolio, nil
}

func (db *Portfolio) Get(ctx context.Context, execution string, date string) (*finance.Portfolio, error) {
	var portfolio finance.Portfolio
	var positions []finance.Position
	var portfolioID int
	err := db.Conn.QueryRow(ctx,
		`SELECT id, cash, value FROM backtest_portfolio WHERE execution = $1 AND date = $2`,
		execution, date).Scan(&portfolioID, &portfolio.Cash, &portfolio.Value)
	if err != nil {
		return nil, err
	}

	rows, err := db.Conn.Query(ctx,
		`SELECT symbol, amount, cost_basis, period FROM backtest_position WHERE portfolio_id = $1`,
		portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var position finance.Position
		err := rows.Scan(&position.Symbol, &position.Amount, &position.CostBasis, &position.Period)
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	portfolio.Positions = positions
	return &portfolio, nil
}
