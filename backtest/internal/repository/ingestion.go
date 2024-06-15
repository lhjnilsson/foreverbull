package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const IngestionTable = `CREATE TABLE IF NOT EXISTS ingestion (
	name text PRIMARY KEY CONSTRAINT ingestionnamecheck CHECK (char_length(name) > 0),
	status text NOT NULL DEFAULT 'CREATED',
	error text,
	
	calendar text NOT NULL DEFAULT 'XNYS',
	start_at TIMESTAMPTZ NOT NULL,
	end_at TIMESTAMPTZ NOT NULL,
	symbols text[]
);

CREATE TABLE IF NOT EXISTS ingestion_status (
	name text REFERENCES ingestion(name) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_ingestion_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update ingestion_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO ingestion_status (name, status, error)
		VALUES (NEW.name, NEW.status, NEW.error);
		PERFORM pg_notify('ingestion_status', NEW.name);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER ingestion_status_trigger AFTER INSERT OR UPDATE ON ingestion
	FOR EACH ROW EXECUTE PROCEDURE notify_ingestion_status();
	EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;`

type Ingestion struct {
	Conn postgres.Query
}

func (db *Ingestion) Create(ctx context.Context, name string, start, end time.Time, calendar string, symbols []string) (*entity.Ingestion, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO ingestion (name, start_at, end_at, calendar, symbols) 
		VALUES ($1, $2, $3, $4, $5)`,
		name, start, end, calendar, symbols)
	if err != nil {
		return nil, err
	}

	return db.Get(ctx, name)
}

func (db *Ingestion) Get(ctx context.Context, name string) (*entity.Ingestion, error) {
	var ingestion entity.Ingestion
	rows, err := db.Conn.Query(ctx,
		`SELECT ingestion.name, start_at, end_at, calendar, symbols,
		ings.status, ings.error, ings.occurred_at
		FROM ingestion
		INNER JOIN (
			SELECT name, status, error, occurred_at
			FROM ingestion_status
			ORDER BY occurred_at DESC
		) AS ings ON ingestion.name=ings.name
		WHERE ingestion.name=$1`,
		name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status entity.IngestionStatus
		err = rows.Scan(&ingestion.Name,
			&ingestion.Start, &ingestion.End, &ingestion.Calendar, &ingestion.Symbols,
			&status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		ingestion.Statuses = append(ingestion.Statuses, status)
	}
	if ingestion.Name == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &ingestion, nil
}

func (db *Ingestion) UpdateStatus(ctx context.Context, name string, status entity.IngestionStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx,
			`UPDATE ingestion SET status=$1, error=$2 WHERE name=$3`,
			status, err.Error(), name)
	} else {
		_, err = db.Conn.Exec(ctx,
			`UPDATE ingestion SET status=$1 WHERE name=$2`,
			status, name)

	}
	return err
}
