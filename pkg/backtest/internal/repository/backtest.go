package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	pb_internal "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
)

const BacktestTable = `CREATE TABLE IF NOT EXISTS backtest (
name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
status int NOT NULL DEFAULT 0,
error text,
start_date date NOT NULL,
end_date date NOT NULL,
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
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO backtest (name, start_date, end_date, symbols, benchmark)
		VALUES ($1, $2, $3, $4, $5)`,
		name, pb_internal.DateToDateString(start), pb_internal.DateToDateString(end), symbols, benchmark)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, name)
}

func (db *Backtest) Get(ctx context.Context, name string) (*pb.Backtest, error) {
	b := pb.Backtest{}
	rows, err := db.Conn.Query(ctx,
		`SELECT backtest.name, start_date, end_date, benchmark, symbols,
		bs.status, bs.error, bs.occurred_at
		FROM backtest
		INNER JOIN (
			SELECT name, status, error, occurred_at FROM backtest_status ORDER BY occurred_at DESC
		) AS bs ON backtest.name=bs.name
		WHERE backtest.name=$1`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		status := pb.Backtest_Status{}
		t := time.Time{}
		start := time.Time{}
		end := time.Time{}
		err = rows.Scan(
			&b.Name, &start, &end, &b.Benchmark, &b.Symbols,
			&status.Status, &status.Error, &t,
		)
		if err != nil {
			return nil, err
		}
		b.StartDate = pb_internal.GoTimeToDate(start)
		b.EndDate = pb_internal.GoTimeToDate(end)
		status.OccurredAt = pb_internal.TimeToProtoTimestamp(t)
		b.Statuses = append(b.Statuses, &status)
	}
	if b.Name == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &b, nil
}

func (db *Backtest) Update(ctx context.Context, name string,
	start, end *pb_internal.Date, symbols []string, benchmark *string) (*pb.Backtest, error) {
	_, err := db.Conn.Exec(ctx,
		`UPDATE backtest SET start_date=$2, end_date=$3,
		symbols=$4, benchmark=$5
		WHERE name=$1`,
		name, pb_internal.DateToDateString(start), pb_internal.DateToDateString(end), symbols, benchmark)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, name)
}

func (db *Backtest) UpdateStatus(ctx context.Context, name string, status pb.Backtest_Status_Status, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, `UPDATE backtest SET status=$2, error=$3 WHERE name=$1`, name, status, err.Error())
	} else {
		_, err = db.Conn.Exec(ctx, `UPDATE backtest SET status=$2 WHERE name=$1`, name, status)
	}
	return err
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
		return nil, err
	}
	defer rows.Close()

	backtests := make([]*pb.Backtest, 0)
	var inReturnSlice bool
	for rows.Next() {
		status := pb.Backtest_Status{}
		b := pb.Backtest{}
		start := time.Time{}
		end := time.Time{}
		occurred_at := time.Time{}
		err = rows.Scan(
			&b.Name, &start, &end, &b.Benchmark,
			&b.Symbols,
			&status.Status, &status.Error, &occurred_at,
		)
		if err != nil {
			return nil, err
		}
		b.StartDate = pb_internal.GoTimeToDate(start)
		b.EndDate = pb_internal.GoTimeToDate(end)
		status.OccurredAt = pb_internal.TimeToProtoTimestamp(occurred_at)
		inReturnSlice = false
		for i := range backtests {
			if backtests[i].Name == b.Name {
				backtests[i].Statuses = append(backtests[i].Statuses, &status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			b.Statuses = append(b.Statuses, &status)
			backtests = append(backtests, &b)
		}
	}
	return backtests, nil
}

func (db *Backtest) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx, `DELETE FROM backtest WHERE name=$1`, name)
	return err
}
