package repository_test

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

func (test *OHLCTests) countBars(symbol string) int {
	var n int
	test.Require().NoError(test.conn.QueryRow(context.Background(),
		`SELECT count(*) FROM ohlc WHERE symbol = $1`, symbol).Scan(&n))
	return n
}

func (test *OHLCTests) TestStoreBarsRefusesOutOfRangeVolumeAndStoresNothing() {
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	err := test.ohlcStorage.StoreBars(context.Background(), "yahoo", "ABC", []*pb.OHLC{
		volumeBar(day, 100),
		volumeBar(day.AddDate(0, 0, 1), uint64(math.MaxInt64)+1),
	})
	test.Require().ErrorIs(err, repository.ErrInvalidPrice)
	test.Require().EqualError(err, "yahoo sent an invalid volume for ABC on 2020-01-03 (out of range)")
	test.Equal(0, test.countBars("ABC"))
}

// noBegin hides the pool's Begin, like a connection that can't open a
// transaction.
type noBegin struct {
	conn postgres.Query
}

func (n noBegin) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return n.conn.Query(ctx, sql, args...)
}

func (n noBegin) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return n.conn.QueryRow(ctx, sql, args...)
}

func (n noBegin) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return n.conn.Exec(ctx, sql, args...)
}

func (test *OHLCTests) TestStoreBarsErrorsWithoutTransaction() {
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	db := repository.OHLC{Conn: noBegin{conn: test.conn}}
	err := db.StoreBars(context.Background(), "yahoo", "ABC", []*pb.OHLC{volumeBar(day, 100)})
	test.Require().ErrorIs(err, repository.ErrNoTransaction)
	test.Equal(0, test.countBars("ABC"))
}
