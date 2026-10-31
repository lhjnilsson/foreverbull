package repository_test

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// Sync gives up on a symbol lock it can't get within lock_timeout, with a
// StoreError, and stores nothing. fetch has already run by then: it runs
// before the lock.
func (test *OHLCTests) TestSyncLockTimeout() {
	defer repository.SetSyncTimeouts("200ms", "90s")()
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	holder, err := test.conn.Begin(ctx)
	test.Require().NoError(err)
	defer holder.Rollback(ctx) //nolint:errcheck
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || 'ABC'))`)
	test.Require().NoError(err)

	started := time.Now()
	err = test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			return []*pb.OHLC{closeBar(day, 125)}, nil
		})
	var stored *repository.StoreError
	test.Require().ErrorAs(err, &stored)
	var pgErr *pgconn.PgError
	test.Require().ErrorAs(err, &pgErr)
	test.Equal("55P03", pgErr.Code, "lock_not_available")
	test.Less(time.Since(started), 5*time.Second)
	test.Zero(test.countRows())
}

// A sync whose transaction sits idle past idle_in_transaction_session_timeout
// is ended by Postgres: nothing is stored and the lock is released. The only
// fetch under the lock is a refetch after the stored history changed between
// the first fetch and the lock, so that refetch is the slow one here.
func (test *OHLCTests) TestSyncIdleInTransactionTimeout() {
	defer repository.SetSyncTimeouts("60s", "200ms")()
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 2, 500)

	var calls atomic.Int32
	err := test.rescaleBeforeTheLock(func() error {
		return test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 2),
			func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
				if calls.Add(1) == 1 {
					return []*pb.OHLC{closeBar(day.AddDate(0, 0, 1), 500), closeBar(day.AddDate(0, 0, 2), 500)}, nil
				}
				time.Sleep(time.Second)
				return []*pb.OHLC{closeBar(day, 125)}, nil
			})
	})
	var stored *repository.StoreError
	test.Require().ErrorAs(err, &stored)
	test.Equal(int32(2), calls.Load())
	test.Equal(map[string]float64{"2020-08-26": 125, "2020-08-27": 125}, test.storedCloses())
	_, err = test.conn.Exec(ctx, `DELETE FROM ohlc`)
	test.Require().NoError(err)

	// The lock is free again: a normal sync goes through.
	restore := repository.SetSyncTimeouts("60s", "90s")
	defer restore()
	test.Require().NoError(test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			return []*pb.OHLC{closeBar(day, 125)}, nil
		}))
	test.Equal(1, test.countRows())
}

// The lock and idle timeouts are transaction-local (SET LOCAL): after a sync
// commits, the same connection is back on the server defaults. The pool has
// one connection, so the SHOWs run on the session the sync used.
func (test *OHLCTests) TestSyncTimeoutsAreTransactionLocal() {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	test.Require().NoError(err)
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	test.Require().NoError(err)
	defer pool.Close()

	show := func() (string, string) {
		var lockTimeout, idleTimeout string
		test.Require().NoError(pool.QueryRow(ctx, `SHOW lock_timeout`).Scan(&lockTimeout))
		test.Require().NoError(pool.QueryRow(ctx, `SHOW idle_in_transaction_session_timeout`).Scan(&idleTimeout))
		return lockTimeout, idleTimeout
	}
	defaultLock, defaultIdle := show()
	test.Require().NotEqual("1min", defaultLock, "the default must differ from Sync's value")
	test.Require().NotEqual("90s", defaultIdle, "the default must differ from Sync's value")

	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	db := repository.OHLC{Conn: pool}
	test.Require().NoError(db.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			return []*pb.OHLC{closeBar(day, 125)}, nil
		}))
	test.Equal(1, test.countRows())

	lockTimeout, idleTimeout := show()
	test.Equal(defaultLock, lockTimeout)
	test.Equal(defaultIdle, idleTimeout)
}
