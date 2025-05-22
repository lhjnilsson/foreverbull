package repository

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

const TradingTable = `
CREATE TABLE IF NOT EXISTS trading_session (
	id text PRIMARY KEY DEFAULT uuid_generate_v4(),
	port integer);
`

type Trading struct {
	Conn postgres.Query
}

func (db *Trading) Create(ctx context.Context) (*pb.Session, error) {
	var id string
	err := db.Conn.QueryRow(ctx, `INSERT INTO trading_session DEFAULT VALUES RETURNING id`).Scan(&id)
	if err != nil {
		return nil, err
	}

	return db.Get(ctx, id)
}

func (db *Trading) Get(ctx context.Context, id string) (*pb.Session, error) {
	session := &pb.Session{}
	err := db.Conn.QueryRow(ctx, `SELECT id, port FROM trading_session WHERE id = $1`, id).Scan(&session.Id, &session.Port)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (db *Trading) UpdatePort(ctx context.Context, id string, port int) error {
	_, err := db.Conn.Exec(ctx, `UPDATE trading_session SET port = $1 WHERE id = $2`, port, id)
	return err
}
