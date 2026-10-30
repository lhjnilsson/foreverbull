package repository_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// symbolLockFree reports whether nobody holds the sync lock of symbol. The
// try-lock runs in its own one-statement transaction, so it is released again
// at once.
func symbolLockFree(ctx context.Context, conn *pgxpool.Pool, symbol string) (bool, error) {
	var free bool
	err := conn.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('ohlc:' || $1))`, symbol).Scan(&free)
	return free, err //nolint:wrapcheck // test helper
}

// waitForLockWaiters waits until n sessions are waiting for an advisory lock
// in this test's database. pg_locks is cluster-wide and the test Postgres
// container is shared by every package's tests, each in its own database, so
// a waiter from another package must not count.
func (test *OHLCTests) waitForLockWaiters(n int) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		test.Require().NoError(test.conn.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_locks
			 WHERE locktype = 'advisory' AND NOT granted
			   AND database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&waiting))
		if waiting == n {
			return
		}
		if time.Now().After(deadline) {
			test.FailNow("advisory lock waiters", "want %d, have %d", n, waiting)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// The provider is asked before the symbol lock is taken, so a slow fetch
// holds neither the lock nor a transaction. Each span is fetched once: the
// locked decision reuses the bars instead of asking again.
func (test *OHLCTests) TestSyncFetchesOutsideTheSymbolLock() {
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 2, 500) // 08-26 and 08-27

	var calls atomic.Int32
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 2),
		func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
			calls.Add(1)
			free, err := symbolLockFree(ctx, test.conn, "ABC")
			test.Require().NoError(err)
			test.True(free, "fetch must run without the symbol lock")
			var open int
			test.Require().NoError(test.conn.QueryRow(ctx,
				// Only this test's database: other packages share the server.
				`SELECT count(*) FROM pg_stat_activity
				  WHERE datname = current_database() AND pid <> pg_backend_pid()
				    AND state LIKE 'idle in transaction%'`).Scan(&open))
			test.Zero(open, "fetch must run outside the sync transaction")
			return []*pb.OHLC{closeBar(day.AddDate(0, 0, 1), 500), closeBar(day.AddDate(0, 0, 2), 501)}, nil
		})
	test.Require().NoError(err)
	test.Equal(int32(1), calls.Load(), "the append span is fetched once")
	test.Equal(map[string]float64{"2020-08-26": 500, "2020-08-27": 500, "2020-08-28": 501}, test.storedCloses())
}

// A replace needs two spans (the incremental fetch whose overlap moved, then
// the whole range). Both are fetched before the lock, once each.
func (test *OHLCTests) TestSyncFetchesEachSpanOnce() {
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 2, 500)

	spans := map[string]int{}
	var underLock atomic.Int32
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 2),
		func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
			spans[from.Format(time.DateOnly)+".."+to.Format(time.DateOnly)]++
			if free, err := symbolLockFree(ctx, test.conn, "ABC"); err == nil && !free {
				underLock.Add(1)
			}
			var out []*pb.OHLC
			for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
				out = append(out, closeBar(d, 125))
			}
			return out, nil
		})
	test.Require().NoError(err)
	test.Equal(map[string]int{"2020-08-27..2020-08-28": 1, "2020-08-26..2020-08-28": 1}, spans)
	test.Zero(underLock.Load())
	test.Equal(map[string]float64{"2020-08-26": 125, "2020-08-27": 125, "2020-08-28": 125}, test.storedCloses())
}

// rescaleBeforeTheLock runs sync while it holds ABC's lock, waits until the
// sync has fetched and is waiting for that lock, re-scales the stored ABC days
// to 125 the way a concurrent post-split replace would, and then lets the
// sync take the lock. It returns the sync's error.
func (test *OHLCTests) rescaleBeforeTheLock(sync func() error) error {
	ctx := context.Background()
	release := test.holdSymbolLocks("ABC")
	defer release()
	done := make(chan error, 1)
	go func() { done <- sync() }()
	test.waitForLockWaiters(1)
	_, err := test.conn.Exec(ctx,
		`UPDATE ohlc SET open = 125, high = 125, low = 125, close = 125 WHERE symbol = 'ABC'`)
	test.Require().NoError(err)
	release()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		test.FailNow("sync did not finish")
		return nil
	}
}

// When another sync changes the stored history between the fetch and the
// lock, the locked decision needs a span that wasn't fetched. Only that span
// is fetched again, under the sync's own lock (the test's hold is released by
// then), and the result is on one scale.
func (test *OHLCTests) TestSyncRefetchesWhenTheDecisionChanged() {
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 2, 500)

	type call struct {
		span      string
		underLock bool
	}
	var mu sync.Mutex
	var calls []call
	err := test.rescaleBeforeTheLock(func() error {
		return test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 2),
			func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
				free, err := symbolLockFree(ctx, test.conn, "ABC")
				if err != nil {
					return nil, err
				}
				mu.Lock()
				calls = append(calls, call{from.Format(time.DateOnly) + ".." + to.Format(time.DateOnly), !free})
				first := len(calls) == 1
				mu.Unlock()
				// The first fetch is on the stored scale (append); the
				// refetch after the re-scale is on the new one.
				c := 125.0
				if first {
					c = 500
				}
				var out []*pb.OHLC
				for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
					out = append(out, closeBar(d, c))
				}
				return out, nil
			})
	})
	test.Require().NoError(err)
	// The first fetch ran while the test held the lock, so only the refetch's
	// lock state says anything: by then only the sync itself can hold it.
	test.Require().Len(calls, 2)
	test.Equal("2020-08-27..2020-08-28", calls[0].span)
	test.Equal(call{"2020-08-26..2020-08-28", true}, calls[1])
	test.Equal(map[string]float64{"2020-08-26": 125, "2020-08-27": 125, "2020-08-28": 125}, test.storedCloses())
}

// A fetch error before the lock comes back unchanged and stores nothing.
func (test *OHLCTests) TestSyncFetchErrorBeforeTheLock() {
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	want := &repository.NoBarsError{Provider: "yahoo", Symbol: "ABC", Start: day}
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
		func(time.Time, *time.Time) ([]*pb.OHLC, error) { return nil, want })
	test.Require().ErrorIs(err, repository.ErrNoBars)
	test.Same(want, err)
	test.Zero(test.countRows())
}
