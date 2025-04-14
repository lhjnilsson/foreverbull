package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

const SessionTable = `CREATE TABLE IF NOT EXISTS session (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	backtest text REFERENCES backtest(name) ON DELETE CASCADE,
	port integer);
`

type Session struct {
	Conn postgres.Query
}

func (db *Session) Create(ctx context.Context, backtest string) (*pb.Session, error) {
	var sessionID string

	err := db.Conn.QueryRow(ctx, `INSERT INTO session (backtest) VALUES ($1) RETURNING id`,
		backtest).Scan(&sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return db.Get(ctx, sessionID)
}

func (db *Session) Get(ctx context.Context, sessionID string) (*pb.Session, error) {
	session := pb.Session{}
	err := db.Conn.QueryRow(ctx,
		`SELECT id, backtest, port WHERE session.id=$1`, sessionID).Scan(
		&session.Id, &session.Backtest, &session.Port,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

func (db *Session) UpdatePort(ctx context.Context, sessionID string, port int) error {
	_, err := db.Conn.Exec(ctx, `UPDATE session SET port=$1 WHERE id=$2`, port, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session port: %w", err)
	}

	return nil
}

func (db *Session) parseRows(rows pgx.Rows) ([]*pb.Session, error) {
	sessions := []*pb.Session{}
	for rows.Next() {
		session := pb.Session{}
		err := rows.Scan(
			&session.Id, &session.Backtest, &session.Port)
		if err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
	}

	return sessions, nil
}

func (db *Session) List(ctx context.Context) ([]*pb.Session, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT id, backtest, port,
		FROM session`)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}

func (db *Session) ListByBacktest(ctx context.Context, backtest string) ([]*pb.Session, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT session.id, backtest, port,
		FROM session
		WHERE backtest=$1`, backtest)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}
