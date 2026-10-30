package repository_test

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// holdSymbolLocks takes the sync lock of each symbol on the suite's own pool,
// so syncs on another pool get a slot and a connection and then wait.
func (test *OHLCTests) holdSymbolLocks(symbols ...string) func() {
	ctx := context.Background()
	holder, err := test.conn.Begin(ctx)
	test.Require().NoError(err)
	for _, symbol := range symbols {
		_, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || $1))`, symbol)
		test.Require().NoError(err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = holder.Rollback(ctx) }) }
}

// Concurrent syncs on one pool are capped at MaxConns-1, so a connection stays
// free for the health Ping while every sync slot is busy. Fetch runs before a
// sync takes a slot, so the syncs are held at the symbol lock instead, where
// each one keeps its connection.
func (test *OHLCTests) TestSyncLeavesAConnectionFree() {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	test.Require().NoError(err)
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	test.Require().NoError(err)
	defer pool.Close()
	test.Require().NoError((&repository.Asset{Conn: pool}).Store(ctx, "XYZ", "Company XYZ"))
	db := repository.OHLC{Conn: pool}
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	// Runs before pool.Close, so a failed check can't leave syncs holding
	// connections that Close would wait for.
	release := test.holdSymbolLocks("ABC", "XYZ")
	defer release()
	done := make(chan error, 2)
	for _, symbol := range []string{"ABC", "XYZ"} {
		go func() {
			done <- db.Sync(ctx, "yahoo", symbol, day, endDay(day, 0),
				func(time.Time, *time.Time) ([]*pb.OHLC, error) {
					return []*pb.OHLC{closeBar(day, 125)}, nil
				})
		}()
	}

	test.waitForLockWaiters(1)
	time.Sleep(300 * time.Millisecond)
	test.waitForLockWaiters(1) // the second sync waits for a slot, not the lock
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	test.Require().NoError(pool.Ping(pingCtx), "a connection must stay free while syncs run")

	release()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			test.Require().NoError(err)
		case <-time.After(10 * time.Second):
			test.FailNow("sync did not finish")
		}
	}
	var rows int
	test.Require().NoError(test.conn.QueryRow(ctx, `SELECT count(*) FROM ohlc`).Scan(&rows))
	test.Equal(2, rows)
}

// A sync waiting for a slot gives up when its context ends. The waiting sync
// is for another symbol (XYZ) than the one holding the slot (ABC), and XYZ's
// lock is free, so it waits on the semaphore: without the cap it would get the
// pool's second connection and store its bars.
func (test *OHLCTests) TestSyncSlotWaitHonoursContext() {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	test.Require().NoError(err)
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	test.Require().NoError(err)
	defer pool.Close()
	test.Require().NoError((&repository.Asset{Conn: pool}).Store(ctx, "XYZ", "Company XYZ"))
	db := repository.OHLC{Conn: pool}
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	release := test.holdSymbolLocks("ABC")
	defer release()
	done := make(chan error, 1)
	go func() {
		done <- db.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
			func(time.Time, *time.Time) ([]*pb.OHLC, error) {
				return []*pb.OHLC{closeBar(day, 125)}, nil
			})
	}()
	test.waitForLockWaiters(1)

	waitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	err = db.Sync(waitCtx, "yahoo", "XYZ", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			return []*pb.OHLC{closeBar(day, 125)}, nil
		})
	var stored *repository.StoreError
	test.Require().ErrorAs(err, &stored)
	test.Require().ErrorIs(err, context.DeadlineExceeded)
	var xyz int
	test.Require().NoError(test.conn.QueryRow(ctx, `SELECT count(*) FROM ohlc WHERE symbol = 'XYZ'`).Scan(&xyz))
	test.Zero(xyz)

	release()
	select {
	case err := <-done:
		test.Require().NoError(err)
	case <-time.After(10 * time.Second):
		test.FailNow("the sync holding the slot did not finish")
	}
}
