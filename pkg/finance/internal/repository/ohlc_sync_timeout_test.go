package repository_test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// Sync gives up on a symbol lock it can't get within lock_timeout, with a
// StoreError, and never calls fetch.
func (test *OHLCTests) TestSyncLockTimeout() {
	defer repository.SetSyncTimeouts("200ms", "90s")()
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	holder, err := test.conn.Begin(ctx)
	test.Require().NoError(err)
	defer holder.Rollback(ctx) //nolint:errcheck
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || 'ABC'))`)
	test.Require().NoError(err)

	fetched := false
	started := time.Now()
	err = test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			fetched = true
			return []*pb.OHLC{closeBar(day, 125)}, nil
		})
	var stored *repository.StoreError
	test.Require().ErrorAs(err, &stored)
	var pgErr *pgconn.PgError
	test.Require().ErrorAs(err, &pgErr)
	test.Equal("55P03", pgErr.Code, "lock_not_available")
	test.Less(time.Since(started), 5*time.Second)
	test.False(fetched)
	test.Zero(test.countRows())
}

// A sync whose transaction sits idle past idle_in_transaction_session_timeout
// (here while fetch is slow) is ended by Postgres: nothing is stored and the
// lock is released.
func (test *OHLCTests) TestSyncIdleInTransactionTimeout() {
	defer repository.SetSyncTimeouts("60s", "200ms")()
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			time.Sleep(time.Second)
			return []*pb.OHLC{closeBar(day, 125)}, nil
		})
	var stored *repository.StoreError
	test.Require().ErrorAs(err, &stored)
	test.Zero(test.countRows())

	// The lock is free again: a normal sync goes through.
	restore := repository.SetSyncTimeouts("60s", "90s")
	defer restore()
	test.Require().NoError(test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			return []*pb.OHLC{closeBar(day, 125)}, nil
		}))
	test.Equal(1, test.countRows())
}
