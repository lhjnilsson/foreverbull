package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	service "github.com/lhjnilsson/foreverbull/service/entity"
)

const ExecutionTable = `CREATE TABLE IF NOT EXISTS execution (
id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
session text REFERENCES session(id) ON DELETE CASCADE,
status text NOT NULL DEFAULT 'CREATED',
error TEXT,
calendar text,
start_at TIMESTAMPTZ,
end_at TIMESTAMPTZ,
benchmark text,
symbols text[],
parameters JSONB);

CREATE TABLE IF NOT EXISTS execution_status (
	id text REFERENCES execution(id) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE OR REPLACE FUNCTION notify_execution_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update execution_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO execution_status (id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('execution_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER execution_status_trigger AFTER INSERT OR UPDATE ON execution
	FOR EACH ROW EXECUTE PROCEDURE notify_execution_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type Execution struct {
	Conn postgres.Query
}

func (db *Execution) Create(ctx context.Context, session, calendar string, start, end time.Time,
	symbols []string, benchmark *string) (*entity.Execution, error) {
	var id string
	err := db.Conn.QueryRow(ctx,
		`INSERT INTO execution (session, calendar, start_at, end_at, benchmark, symbols)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, session, calendar, start, end, benchmark, symbols).
		Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, id)
}

func (db *Execution) Get(ctx context.Context, id string) (*entity.Execution, error) {
	e := entity.Execution{}
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session, calendar, start_at, end_at, benchmark, symbols, parameters,
		es.status, es.error, es.occurred_at
		FROM execution 
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		WHERE execution.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		status := entity.ExecutionStatus{}
		err = rows.Scan(&e.ID, &e.Session, &e.Calendar, &e.Start, &e.End, &e.Benchmark, &e.Symbols,
			&e.Parameters, &status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		e.Statuses = append(e.Statuses, status)
	}
	if e.ID == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &e, nil
}

func (db *Execution) UpdateSimulationDetails(ctx context.Context, id string, calendar string, Start time.Time,
	End time.Time, benchmark *string, symbols []string) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE execution SET calendar=$1, start_at=$2, end_at=$3, benchmark=$4, 
		symbols=$5 WHERE id=$6`, calendar, Start, End, benchmark, symbols, id)
	return err
}

func (db *Execution) UpdateParameters(ctx context.Context, id string, parameters *[]service.Parameter) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE execution SET parameters=$1 WHERE id=$2`, parameters, id)
	return err
}

func (db *Execution) UpdateStatus(ctx context.Context, id string, status entity.ExecutionStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, "UPDATE execution SET status=$2, error=$3 WHERE id=$1", id, status, err.Error())
	} else {
		_, err = db.Conn.Exec(ctx, "UPDATE execution SET status=$2 WHERE id=$1", id, status)
	}
	return err
}

func (db *Execution) List(ctx context.Context) (*[]entity.Execution, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session, calendar, start_at, end_at, benchmark, symbols, parameters,
		es.status, es.error, es.occurred_at
		FROM execution
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		ORDER BY es.occurred_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := make([]entity.Execution, 0)
	var inReturnSlice bool
	for rows.Next() {
		status := entity.ExecutionStatus{}
		e := entity.Execution{}
		err = rows.Scan(&e.ID, &e.Session, &e.Calendar, &e.Start, &e.End, &e.Benchmark, &e.Symbols,
			&e.Parameters, &status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range executions {
			if executions[i].ID == e.ID {
				executions[i].Statuses = append(executions[i].Statuses, status)
				inReturnSlice = true
				break
			}
		}
		if !inReturnSlice {
			e.Statuses = append(e.Statuses, status)
			executions = append(executions, e)
		}
	}
	return &executions, nil
}

func (db *Execution) ListBySession(ctx context.Context, session string) (*[]entity.Execution, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session, calendar, start_at, end_at, benchmark, symbols, parameters,
		es.status, es.error, es.occurred_at
		FROM execution
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		WHERE session=$1
		ORDER BY es.occurred_at DESC`, session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := make([]entity.Execution, 0)
	var inReturnSlice bool
	for rows.Next() {
		status := entity.ExecutionStatus{}
		e := entity.Execution{}
		err = rows.Scan(&e.ID, &e.Session, &e.Calendar, &e.Start, &e.End, &e.Benchmark, &e.Symbols,
			&e.Parameters, &status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range executions {
			if executions[i].ID == e.ID {
				executions[i].Statuses = append(executions[i].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			e.Statuses = append(e.Statuses, status)
			executions = append(executions, e)
		}
	}
	return &executions, nil
}
