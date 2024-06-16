package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
)

const SessionTable = `CREATE TABLE IF NOT EXISTS session (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	backtest text REFERENCES backtest(name) ON DELETE CASCADE,
	manual boolean NOT NULL DEFAULT false,
	status text NOT NULL DEFAULT 'CREATED',
	error text,
	port integer);

CREATE TABLE IF NOT EXISTS session_status (
	id text REFERENCES session(id) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_session_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update session_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO session_status (id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('session_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER session_status_trigger AFTER INSERT OR UPDATE ON session
	FOR EACH ROW EXECUTE PROCEDURE notify_session_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type Session struct {
	Conn postgres.Query
}

func (db *Session) Create(ctx context.Context, backtest string, manual bool) (*entity.Session, error) {
	var id string
	err := db.Conn.QueryRow(ctx, `INSERT INTO session (backtest, manual) VALUES ($1, $2) RETURNING id`,
		backtest, manual).Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.Get(ctx, id)
}

func (db *Session) Get(ctx context.Context, id string) (*entity.Session, error) {
	s := entity.Session{}
	rows, err := db.Conn.Query(ctx,
		`SELECT session.id, backtest, manual, port,
		(SELECT COUNT(*) FROM execution WHERE session=id) AS executions,
		ss.status, ss.error, ss.occurred_at
		FROM session
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM session_status ORDER BY occurred_at DESC
		) AS ss ON session.id=ss.id
		WHERE session.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		status := entity.SessionStatus{}
		err = rows.Scan(
			&s.ID, &s.Backtest, &s.Manual, &s.Port, &s.Executions,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		s.Statuses = append(s.Statuses, status)
	}
	if s.ID == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &s, nil
}

func (db *Session) UpdateStatus(ctx context.Context, id string, status entity.SessionStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, `UPDATE session SET status=$1, error=$2 WHERE id=$3`, status, err.Error(), id)
	} else {
		_, err = db.Conn.Exec(ctx, `UPDATE session SET status=$1, error=NULL WHERE id=$2`, status, id)
	}
	return err
}

func (db *Session) UpdatePort(ctx context.Context, id string, port int) error {
	_, err := db.Conn.Exec(ctx, `UPDATE session SET port=$1 WHERE id=$2`, port, id)
	return err
}

func (db *Session) List(ctx context.Context) (*[]entity.Session, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT session.id, backtest, manual, port,
		(SELECT COUNT(*) FROM execution WHERE session=id) AS executions,
		ss.status, ss.error, ss.occurred_at
		FROM session
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM session_status ORDER BY occurred_at DESC
		) AS ss ON session.id=ss.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []entity.Session{}
	var inReturnSlice bool
	for rows.Next() {
		status := entity.SessionStatus{}
		s := entity.Session{}
		err = rows.Scan(
			&s.ID, &s.Backtest, &s.Manual, &s.Port, &s.Executions,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range sessions {
			if sessions[i].ID == s.ID {
				sessions[i].Statuses = append(sessions[i].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			s.Statuses = append(s.Statuses, status)
			sessions = append(sessions, s)
		}
	}
	return &sessions, nil
}

func (db *Session) ListByBacktest(ctx context.Context, backtest string) (*[]entity.Session, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT session.id, backtest, manual, port,
		(SELECT COUNT(*) FROM execution WHERE session=id) AS executions,
		ss.status, ss.error, ss.occurred_at
		FROM session
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM session_status ORDER BY occurred_at DESC
		) AS ss ON session.id=ss.id
		WHERE backtest=$1`, backtest)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []entity.Session{}
	var inReturnSlice bool
	for rows.Next() {
		status := entity.SessionStatus{}
		s := entity.Session{}
		err = rows.Scan(
			&s.ID, &s.Backtest, &s.Manual, &s.Port, &s.Executions,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range sessions {
			if sessions[i].ID == s.ID {
				sessions[i].Statuses = append(sessions[i].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			s.Statuses = append(s.Statuses, status)
			sessions = append(sessions, s)
		}
	}
	return &sessions, nil
}
