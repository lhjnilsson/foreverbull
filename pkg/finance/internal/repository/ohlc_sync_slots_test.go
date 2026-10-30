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

// Concurrent syncs on one pool are capped at MaxConns-1, so a connection stays
// free for the health Ping while every sync slot is busy.
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

	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	// Runs before pool.Close, so a failed check can't leave syncs holding
	// connections that Close would wait for.
	defer releaseAll()
	entered := make(chan string, 2)
	done := make(chan error, 2)
	for _, symbol := range []string{"ABC", "XYZ"} {
		go func() {
			done <- db.Sync(ctx, "yahoo", symbol, day, endDay(day, 0),
				func(time.Time, *time.Time) ([]*pb.OHLC, error) {
					entered <- symbol
					<-release
					return []*pb.OHLC{closeBar(day, 125)}, nil
				})
		}()
	}

	first := <-entered
	select {
	case second := <-entered:
		test.FailNow("both syncs ran at once on a 2-connection pool", "%s and %s", first, second)
	case <-time.After(300 * time.Millisecond):
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	test.Require().NoError(pool.Ping(pingCtx), "a connection must stay free while syncs run")

	releaseAll()
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

// A sync waiting for a slot gives up when its context ends.
func (test *OHLCTests) TestSyncSlotWaitHonoursContext() {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	test.Require().NoError(err)
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	test.Require().NoError(err)
	defer pool.Close()
	db := repository.OHLC{Conn: pool}
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- db.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
			func(time.Time, *time.Time) ([]*pb.OHLC, error) {
				close(entered)
				<-release
				return []*pb.OHLC{closeBar(day, 125)}, nil
			})
	}()
	<-entered

	waitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	err = db.Sync(waitCtx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) {
			test.FailNow("fetch must not run without a slot")
			return nil, nil
		})
	var stored *repository.StoreError
	test.Require().ErrorAs(err, &stored)
	test.Require().ErrorIs(err, context.DeadlineExceeded)

	releaseAll()
	test.Require().NoError(<-done)
}
