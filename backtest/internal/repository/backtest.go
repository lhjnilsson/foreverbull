package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const BacktestTable = `CREATE TABLE IF NOT EXISTS backtest (
name text PRIMARY KEY CONSTRAINT backtestnamecheck CHECK (char_length(name) > 0),
status text NOT NULL DEFAULT 'CREATED',
error text,
service text,
calendar text NOT NULL DEFAULT 'XNYS',
start_at TIMESTAMPTZ NOT NULL,
end_at TIMESTAMPTZ NOT NULL,
benchmark text,
symbols text[]);

CREATE TABLE IF NOT EXISTS backtest_status (
	name text REFERENCES backtest(name) ON DELETE CASCADE,
	status text NOT NULL,
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

func (db *Backtest) Create(ctx context.Context, name string, service *string,
	start, end time.Time, calendar string, symbols []string, benchmark *string) (*entity.Backtest, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO backtest (name, service, start_at, end_at, calendar, symbols, benchmark) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		name, service, start, end, calendar, symbols, benchmark)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, name)
}

func (db *Backtest) Get(ctx context.Context, name string) (*entity.Backtest, error) {
	b := entity.Backtest{}
	rows, err := db.Conn.Query(ctx,
		`SELECT backtest.name, service,
		calendar, start_at, end_at, benchmark, symbols,
		(SELECT count(*) FROM session WHERE backtest=backtest.name),
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
		status := entity.BacktestStatus{}
		err = rows.Scan(
			&b.Name, &b.Service,
			&b.Calendar, &b.Start, &b.End, &b.Benchmark, &b.Symbols,
			&b.Sessions,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		b.Statuses = append(b.Statuses, status)
	}
	if b.Name == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &b, nil
}

func (db *Backtest) Update(ctx context.Context, name string, service *string,
	start, end time.Time, calendar string, symbols []string, benchmark *string) (*entity.Backtest, error) {
	_, err := db.Conn.Exec(ctx,
		`UPDATE backtest SET service=$2, start_at=$3, end_at=$4, 
		calendar=$5, symbols=$6, benchmark=$7, status='UPDATED'
		WHERE name=$1`,
		name, service, start, end, calendar, symbols, benchmark)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, name)
}

func (db *Backtest) UpdateStatus(ctx context.Context, name string, status entity.BacktestStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, `UPDATE backtest SET status=$2, error=$3 WHERE name=$1`, name, status, err.Error())
	} else {
		_, err = db.Conn.Exec(ctx, `UPDATE backtest SET status=$2 WHERE name=$1`, name, status)
	}
	return err
}

func (db *Backtest) List(ctx context.Context) (*[]entity.Backtest, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT backtest.name, service,
		calendar, start_at, end_at, benchmark, symbols,
		(SELECT count(*) FROM session WHERE backtest=backtest.name),
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

	backtests := []entity.Backtest{}
	var inReturnSlice bool
	for rows.Next() {
		status := entity.BacktestStatus{}
		b := entity.Backtest{}
		err = rows.Scan(
			&b.Name, &b.Service,
			&b.Calendar, &b.Start, &b.End, &b.Benchmark, &b.Symbols,
			&b.Sessions,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range backtests {
			if backtests[i].Name == b.Name {
				backtests[i].Statuses = append(backtests[i].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			b.Statuses = append(b.Statuses, status)
			backtests = append(backtests, b)
		}
	}
	return &backtests, nil
}

func (db *Backtest) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx, `DELETE FROM backtest WHERE name=$1`, name)
	return err
}
