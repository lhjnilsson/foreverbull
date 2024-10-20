package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	pb_internal "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
)

const BacktestTable = `CREATE TABLE IF NOT EXISTS backtest (
name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
status int NOT NULL DEFAULT 0,
error text,
start_date date NOT NULL,
end_date date,
benchmark text,
symbols text[]);

CREATE TABLE IF NOT EXISTS backtest_status (
	name text REFERENCES backtest(name) ON DELETE CASCADE,
	status int NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_backtest_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update backtest_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO backtest_status (name, status, error)
		VALUES (NEW.name, NEW.status, NEW.error);
		PERFORM pg_notify('backtest_status', NEW.name);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER backtest_status_trigger AFTER INSERT OR UPDATE ON backtest
	FOR EACH ROW EXECUTE PROCEDURE notify_backtest_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type Backtest struct {
	Conn postgres.Query
}

func (db *Backtest) Create(ctx context.Context, name string,
	start, end *pb_internal.Date, symbols []string, benchmark *string) (*pb.Backtest, error) {
	var endDate *string

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

func (db *Backtest) Get(ctx context.Context, name string) (*pb.Backtest, error) {
	backtest := pb.Backtest{}

	rows, err := db.Conn.Query(ctx,
		`SELECT backtest.name, start_date, end_date, benchmark, symbols,
		bs.status, bs.error, bs.occurred_at
		FROM backtest
		INNER JOIN (
			SELECT name, status, error, occurred_at FROM backtest_status ORDER BY occurred_at DESC
		) AS bs ON backtest.name=bs.name
		WHERE backtest.name=$1`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		status := pb.Backtest_Status{}
		occuredAt := time.Time{}
		start := time.Time{}
		endDate := pgtype.Date{}

		err = rows.Scan(
			&backtest.Name, &start, &endDate, &backtest.Benchmark, &backtest.Symbols,
			&status.Status, &status.Error, &occuredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backtest: %w", err)
		}

		backtest.StartDate = pb_internal.GoTimeToDate(start)
		if endDate.Valid {
			backtest.EndDate = pb_internal.GoTimeToDate(endDate.Time)
		}

		status.OccurredAt = pb_internal.TimeToProtoTimestamp(occuredAt)
		backtest.Statuses = append(backtest.Statuses, &status)
	}

	if backtest.Name == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}

	return &backtest, nil
}

func (db *Backtest) GetUniverse(ctx context.Context) (*pb_internal.Date, *pb_internal.Date, []string, error) {
	var startDate, endDate pgtype.Date

	var symbols []string
	err := db.Conn.QueryRow(ctx,
		`WITH symbols_unnested AS (
    SELECT unnest(symbols) AS symbol
    FROM backtest
),
all_symbols AS (
    SELECT symbol FROM symbols_unnested
    UNION
    SELECT benchmark AS symbol FROM backtest WHERE benchmark IS NOT NULL
)
SELECT
    MIN(start_date) AS min_start_date,
    CASE
        WHEN COUNT(*) FILTER (WHERE end_date IS NULL) > 0 THEN CURRENT_DATE
        ELSE MAX(end_date)
    END AS max_date,
    ARRAY_AGG(DISTINCT symbol) AS distinct_symbols
FROM backtest, all_symbols`).Scan(&startDate, &endDate, &symbols)

	if !startDate.Valid || !endDate.Valid {
		return nil, nil, nil, fmt.Errorf("failed to get universe: %w", err)
	}

	return pb_internal.GoTimeToDate(startDate.Time), pb_internal.GoTimeToDate(endDate.Time), symbols, err
}

func (db *Backtest) Update(ctx context.Context, name string,
	start, end *pb_internal.Date, symbols []string, benchmark *string) (*pb.Backtest, error) {
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

func (db *Backtest) UpdateStatus(ctx context.Context, name string, status pb.Backtest_Status_Status, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, `UPDATE backtest SET status=$2, error=$3 WHERE name=$1`, name, status, err.Error())
	} else {
		_, err = db.Conn.Exec(ctx, `UPDATE backtest SET status=$2 WHERE name=$1`, name, status)
	}

	if err != nil {
		return fmt.Errorf("failed to update backtest status: %w", err)
	}

	return nil
}

func (db *Backtest) List(ctx context.Context) ([]*pb.Backtest, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT backtest.name, start_date, end_date, benchmark, symbols,
		bs.status, bs.error, bs.occurred_at
		FROM backtest
		INNER JOIN (
			SELECT name, status, error, occurred_at FROM backtest_status ORDER BY occurred_at DESC
		) AS bs ON backtest.name=bs.name
		ORDER BY bs.occurred_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list backtests: %w", err)
	}
	defer rows.Close()

	backtests := make([]*pb.Backtest, 0)

	var inReturnSlice bool

	for rows.Next() {
		status := pb.Backtest_Status{}
		backtest := pb.Backtest{}
		start := time.Time{}
		end := pgtype.Date{}
		occurred_at := time.Time{}

		err = rows.Scan(
			&backtest.Name, &start, &end, &backtest.Benchmark,
			&backtest.Symbols,
			&status.Status, &status.Error, &occurred_at,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backtest: %w", err)
		}

		backtest.StartDate = pb_internal.GoTimeToDate(start)
		if end.Valid {
			backtest.EndDate = pb_internal.GoTimeToDate(end.Time)
		}

		status.OccurredAt = pb_internal.TimeToProtoTimestamp(occurred_at)
		inReturnSlice = false

		for i := range backtests {
			if backtests[i].Name == backtest.Name {
				backtests[i].Statuses = append(backtests[i].Statuses, &status)
				inReturnSlice = true
			}
		}

		if !inReturnSlice {
			backtest.Statuses = append(backtest.Statuses, &status)
			backtests = append(backtests, &backtest)
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
