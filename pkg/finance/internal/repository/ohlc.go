package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
)

const OHLCTable = `CREATE TABLE IF NOT EXISTS ohlc (
symbol TEXT references asset(symbol),
open numeric,
high numeric,
low numeric,
close numeric,
volume bigint,
time timestamp,
UNIQUE (symbol, time));
`

type OHLC struct {
	Conn postgres.Query
}

func (db *OHLC) Store(ctx context.Context, symbol string, t time.Time, o, h, l, c float64, v int) error {
	_, err := db.Conn.Exec(
		ctx,
		`INSERT into ohlc(symbol, time, open, high, low, close, volume)
		values ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`, symbol, t, o, h, l, c, v)
	if err != nil {
		return fmt.Errorf("failed to store ohlc: %w", err)
	}

	return nil
}

func (db *OHLC) Exists(ctx context.Context, symbols []string, start, end *pb.Date) (bool, error) {
	var startExists bool

	var endExists bool

	for _, symbol := range symbols {
		err := db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE symbol = $1 AND time::date = $2)",
			symbol, pb.DateToDateString(start)).Scan(&startExists)
		if err != nil {
			return false, fmt.Errorf("failed to check if ohlc exists: %w", err)
		}

		err = db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE symbol = $1 AND time::date = $2)",
			symbol, pb.DateToDateString(end)).Scan(&endExists)
		if err != nil {
			return false, fmt.Errorf("failed to check if ohlc exists: %w", err)
		}

		if !startExists || !endExists {
			return false, nil
		}
	}

	return true, nil
}

func (db *OHLC) MinMax(ctx context.Context) (*pb.Date, *pb.Date, error) {
	var minTime *time.Time

	var maxTime *time.Time

	err := db.Conn.QueryRow(
		ctx,
		"SELECT MIN(time), MAX(time) FROM ohlc").Scan(&minTime, &maxTime)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get min/max time: %w", err)
	}

	if minTime == nil || maxTime == nil {
		return nil, nil, nil
	}

	return pb.GoTimeToDate(*minTime), pb.GoTimeToDate(*maxTime), nil
}
