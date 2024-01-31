package repository

import (
	"context"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const OrderTable = `CREATE TABLE IF NOT EXISTS backtest_order (
id text primary key,
execution text NOT NULL,
symbol TEXT,
amount INTEGER,
limit_price numeric,
stop_price numeric,
filled INTEGER,
commission numeric,
status INTEGER,
created_at TIMESTAMP
);
`

type Order struct {
	Conn postgres.Query
}

/*
List
Lists all orders of a session
*/
func (db *Order) List(ctx context.Context, execution string) (*[]finance.Order, error) {
	rows, err := db.Conn.Query(
		ctx,
		`SELECT id, symbol, amount, limit_price, stop_price, filled, commission, 
		status, created_at FROM backtest_order WHERE execution=$1`, execution)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]finance.Order, 0)
	for rows.Next() {
		order := finance.Order{}
		err = rows.Scan(&order.ID, &order.Symbol, &order.Amount, &order.LimitPrice,
			&order.StopPrice, &order.Filled, &order.Commission, &order.Status, &order.CreatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return &orders, nil
}

/*
Store
Saves a new order
*/
func (db *Order) Store(ctx context.Context, execution string, order *finance.Order) error {
	_, err := db.Conn.Exec(
		ctx,
		`INSERT INTO backtest_order(id, symbol, execution, amount, limit_price, stop_price, 
			filled, commission, status, created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT(id) DO UPDATE SET amount=$4, limit_price=$5, stop_price=$6, filled=$7, commission=$8, status=$9, created_at=$10`,
		order.ID, order.Symbol, execution, order.Amount,
		order.LimitPrice, order.StopPrice, order.Filled, order.Commission, order.Status, order.CreatedAt)
	return err
}
