package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	pb_internal "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

const BacktestTable = `CREATE TABLE IF NOT EXISTS backtest (
name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
start_date date NOT NULL,
end_date date,
benchmark text,
symbols text[]);
`

type Backtest struct {
	Conn postgres.Query
}

func (db *Backtest) Create(ctx context.Context, name string,
	start, end *pb_internal.Date, symbols []string, benchmark *string,
) (*backtest.Backtest, error) {
	var endDate *string

	if start == nil {
		return nil, fmt.Errorf("failed to create backtest: start date is nil")
	}
	if end != nil {
		ed := pb_internal.DateToDateString(end)
		endDate = &ed
	}

	_, err := db.Conn.Exec(ctx,
		`INSERT INTO backtest (name, start_date, end_date, symbols, benchmark)
		VALUES ($1, $2, $3, $4, $5)`,
		name, pb_internal.DateToDateString(start), endDate, symbols, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to create backtest: %w", err)
	}

	return db.Get(ctx, name)
}

func (db *Backtest) Get(ctx context.Context, name string) (*backtest.Backtest, error) {
	b := backtest.Backtest{}
	startDate := time.Time{}
	endDate := time.Time{}

	err := db.Conn.QueryRow(ctx,
		`SELECT backtest.name, start_date, end_date, benchmark, symbols
		FROM backtest
		WHERE backtest.name=$1`, name).Scan(&b.Name, &startDate, &endDate, &b.Benchmark, &b.Symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest: %w", err)
	}
	b.StartDate = pb_internal.GoTimeToDate(startDate)
	b.EndDate = pb_internal.GoTimeToDate(endDate)

	return &b, nil
}

func (db *Backtest) Update(ctx context.Context, name string,
	start, end *pb_internal.Date, symbols []string, benchmark *string,
) (*backtest.Backtest, error) {
	_, err := db.Conn.Exec(ctx,
		`UPDATE backtest SET start_date=$2, end_date=$3,
		symbols=$4, benchmark=$5
		WHERE name=$1`,
		name, pb_internal.DateToDateString(start), pb_internal.DateToDateString(end), symbols, benchmark)
	if err != nil {
		return nil, fmt.Errorf("failed to update backtest: %w", err)
	}

	return db.Get(ctx, name)
}

func (db *Backtest) List(ctx context.Context) ([]*backtest.Backtest, error) {
	rows, err := db.Conn.Query(ctx, `SELECT name, start_date, end_date, benchmark, symbols FROM backtest`)
	if err != nil {
		return nil, fmt.Errorf("failed to list backtests: %w", err)
	}
	defer rows.Close()

	backtests := make([]*backtest.Backtest, 0)

	for rows.Next() {
		backtest := backtest.Backtest{}
		start := time.Time{}
		end := pgtype.Date{}

		err = rows.Scan(
			&backtest.Name, &start, &end, &backtest.Benchmark,
			&backtest.Symbols,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backtest: %w", err)
		}

		backtest.StartDate = pb_internal.GoTimeToDate(start)
		if end.Valid {
			backtest.EndDate = pb_internal.GoTimeToDate(end.Time)
		}
	}

	return backtests, nil
}

func (db *Backtest) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx, `DELETE FROM backtest WHERE name=$1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete backtest: %w", err)
	}

	return nil
}
