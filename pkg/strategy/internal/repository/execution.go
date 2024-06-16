package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	finance "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/entity"
)

const ExecutionTable = `
CREATE TABLE IF NOT EXISTS strategy_execution (
id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
strategy text REFERENCES strategy(name) ON DELETE CASCADE,
service text NOT NULL,
status text NOT NULL DEFAULT 'CREATED',
error TEXT,
start_at TIMESTAMPTZ NOT NULL,
end_at TIMESTAMPTZ NOT NULL,
portfolio JSONB NOT NULL DEFAULT '{}',
orders JSONB NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS strategy_execution_status (
	id text REFERENCES strategy_execution(id) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
	
CREATE OR REPLACE FUNCTION notify_strategy_execution_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update strategy_execution_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO strategy_execution_status (id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('strategy_execution_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;
	
DO
$$BEGIN
	CREATE TRIGGER strategy_execution_status_trigger AFTER INSERT OR UPDATE ON strategy_execution
	FOR EACH ROW EXECUTE PROCEDURE notify_strategy_execution_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type Execution struct {
	Conn postgres.Query
}

func (db *Execution) Create(ctx context.Context, strategy string, start, end time.Time, service string) (*entity.Execution, error) {
	var id string
	err := db.Conn.QueryRow(ctx,
		`INSERT INTO strategy_execution (strategy, start_at, end_at, service) VALUES ($1, $2, $3, $4) RETURNING id`,
		strategy, start, end, service).Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, id)
}

func (db *Execution) UpdateStatus(ctx context.Context, id string, status entity.ExecutionStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, `UPDATE strategy_execution SET status=$1, error=$2 WHERE id=$3`, status, err.Error(), id)
	} else {
		_, err = db.Conn.Exec(ctx, `UPDATE strategy_execution SET status=$1 WHERE id=$2`, status, id)
	}
	return err
}

func (db *Execution) List(ctx context.Context, strategy string) (*[]entity.Execution, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, strategy, start_at, end_at, service, portfolio, orders, es.status, es.error, es.occurred_at
	FROM strategy_execution AS execution
	INNER JOIN (
		SELECT id, status, error, occurred_at FROM strategy_execution_status ORDER BY occurred_at DESC
	) AS es ON execution.id=es.id
	WHERE strategy=$1`, strategy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executions := []entity.Execution{}
	var inReturnSlice bool
	for rows.Next() {
		e := entity.Execution{}
		status := entity.ExecutionStatus{}
		err = rows.Scan(&e.ID, &e.Strategy, &e.Start, &e.End, &e.Service, &e.StartPortfolio, &e.PlacedOrders, &status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for _, ex := range executions {
			if ex.ID == e.ID {
				inReturnSlice = true
				ex.Statuses = append(ex.Statuses, status)
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

func (db *Execution) Get(ctx context.Context, id string) (*entity.Execution, error) {
	e := entity.Execution{}
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, strategy, start_at, end_at, service, portfolio, orders,  es.status, es.error, es.occurred_at
	FROM strategy_execution AS execution
	INNER JOIN (
		SELECT id, status, error, occurred_at FROM strategy_execution_status ORDER BY occurred_at DESC
	) AS es ON execution.id=es.id
	WHERE execution.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		status := entity.ExecutionStatus{}
		err = rows.Scan(&e.ID, &e.Strategy, &e.Start, &e.End, &e.Service,
			&e.StartPortfolio, &e.PlacedOrders,
			&status.Status, &status.Error, &status.OccurredAt)
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

func (db *Execution) SetStartPortfolio(ctx context.Context, id string, portfolio *finance.Portfolio) error {
	_, err := db.Conn.Exec(ctx, `UPDATE strategy_execution SET portfolio=$1 WHERE id=$2`, portfolio, id)
	return err
}

func (db *Execution) SetPlacedOrders(ctx context.Context, id string, orders *[]finance.Order) error {
	_, err := db.Conn.Exec(ctx, `UPDATE strategy_execution SET orders=$1 WHERE id=$2`, orders, id)
	return err
}
