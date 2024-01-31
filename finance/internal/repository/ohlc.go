package repository

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const OHLCTable = `CREATE TABLE IF NOT EXISTS ohlc (
symbol TEXT references asset(symbol),
open numeric,
high numeric,
low numeric,
close numeric,
volume integer,
time timestamp,
UNIQUE (symbol, time));
`

type OHLC struct {
	Conn postgres.Query
}

/*
Create
Create new End-of-day entry, based on instrument and session
*/
func (db *OHLC) Store(ctx context.Context, symbol string, ohlc *entity.OHLC) error {
	_, err := db.Conn.Exec(
		ctx,
		`INSERT into ohlc(symbol, time, open, high, low, close, volume) 
		values ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`, symbol, ohlc.Time, ohlc.Open, ohlc.High, ohlc.Low, ohlc.Close, ohlc.Volume)
	return err
}

/*
Exists
Check if OHLC entry exists for instrument
*/
func (db *OHLC) Exists(ctx context.Context, symbols []string, Start time.Time, End time.Time) (bool, error) {
	var startExists bool
	var endExists bool
	for _, symbol := range symbols {
		err := db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE symbol = $1 AND time = $2::date)",
			symbol, Start).Scan(&startExists)
		if err != nil {
			return false, err
		}
		err = db.Conn.QueryRow(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM ohlc WHERE symbol = $1 AND time = $2::date)",
			symbol, End).Scan(&endExists)
		if err != nil {
			return false, err
		}
		if !startExists || !endExists {
			return false, nil
		}
	}
	return true, nil
}

func (db *OHLC) MinMax(ctx context.Context) (*time.Time, *time.Time, error) {
	var minTime *time.Time
	var maxTime *time.Time
	err := db.Conn.QueryRow(
		ctx,
		"SELECT MIN(time), MAX(time) FROM ohlc").Scan(&minTime, &maxTime)
	if err != nil {
		return nil, nil, err
	}
	return minTime, maxTime, nil
}
