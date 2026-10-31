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

// AAPL closes around its 4:1 split on 2020-08-31, before and after Yahoo
// adjusted them. 499.23 / 4 = 124.8075.
var (
	aaplPreSplit = map[string]float64{"2020-08-26": 506.09, "2020-08-27": 500.04, "2020-08-28": 499.23}
	aaplAdjusted = map[string]float64{"2020-08-26": 126.5225, "2020-08-27": 125.01, "2020-08-28": 124.8075}
)

// aaplBars returns the closes for every day from through to.
func aaplBars(closes map[string]float64, from time.Time, to *time.Time) []*pb.OHLC {
	var out []*pb.OHLC
	for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
		if c, ok := closes[d.Format(time.DateOnly)]; ok {
			out = append(out, closeBar(d, c))
		}
	}
	return out
}

func (test *OHLCTests) aaplCloses() map[string]float64 {
	rows, err := test.conn.Query(context.Background(),
		`SELECT time, close::float8 FROM ohlc WHERE provider = 'yahoo' AND symbol = 'AAPL' ORDER BY time`)
	test.Require().NoError(err)
	defer rows.Close()
	got := map[string]float64{}
	for rows.Next() {
		var t time.Time
		var c float64
		test.Require().NoError(rows.Scan(&t, &c))
		got[t.Format(time.DateOnly)] = c
	}
	test.Require().NoError(rows.Err())
	return got
}

// Golden check for the fetch-before-lock order: AAPL is stored on the
// pre-split scale, and sync A fetches the next days early while the provider
// still answers on that scale. After that fetch and before A takes the symbol
// lock, a second sync (B) runs to the end with the adjusted history and
// replaces every stored day. A must not append its stale bars next to B's:
// under the lock it sees the overlap day moved, fetches the whole range
// again, and the result is AAPL 2020-08-28 at 124.8075 with every row on the
// adjusted scale.
//
// A runs on a pool with one sync slot, held by a sync of another symbol that
// waits for a lock the test holds. So A fetches, then waits for the slot,
// which it gets only after B has finished.
func (test *OHLCTests) TestSyncAfterAnotherSyncRescaledBeforeTheLock() {
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.Run("B syncs the same range", func() {
		// B's replace stores through 08-28, so under the lock A finds the
		// range stored and writes none of its stale bars.
		test.SetupTest()
		calls := test.syncWhileAnotherSyncRescales(day, endDay(day, 2))
		test.Equal([]syncCall{{"2020-08-27..2020-08-28", false}}, calls)
	})
	test.Run("B syncs an earlier range", func() {
		// B replaces 08-25 through 08-27 only, so under the lock A's
		// fetched overlap day (500.04) no longer matches the stored one
		// (125.01): A fetches the whole range again under its own lock.
		test.SetupTest()
		calls := test.syncWhileAnotherSyncRescales(day.AddDate(0, 0, -1), endDay(day, 1))
		test.Equal([]syncCall{{"2020-08-27..2020-08-28", false}, {"2020-08-26..2020-08-28", true}}, calls)
	})
}

type syncCall struct {
	span      string
	underLock bool
}

// syncWhileAnotherSyncRescales runs sync A for AAPL 08-26 through 08-28 over
// pre-split rows and runs sync B (adjusted, bStart through bEnd) between A's
// early fetch and A's lock. It checks the golden end state and returns A's
// fetches.
func (test *OHLCTests) syncWhileAnotherSyncRescales(bStart time.Time, bEnd *time.Time) []syncCall {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	test.Require().NoError(err)
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	test.Require().NoError(err)
	defer pool.Close()
	slow := repository.OHLC{Conn: pool}

	assets := repository.Asset{Conn: test.conn}
	test.Require().NoError(assets.Store(ctx, "AAPL", "Apple Inc."))
	test.Require().NoError(assets.Store(ctx, "XYZ", "Company XYZ"))
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	for _, d := range []time.Time{day, day.AddDate(0, 0, 1)} {
		c := aaplPreSplit[d.Format(time.DateOnly)]
		test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "AAPL", d, c, c, c, c, 100))
	}
	end := endDay(day, 2)

	// Runs before pool.Close, so a failed check can't leave syncs holding
	// connections that Close would wait for.
	release := test.holdSymbolLocks("XYZ")
	defer release()
	slotHolder := make(chan error, 1)
	go func() {
		slotHolder <- slow.Sync(ctx, "yahoo", "XYZ", day, endDay(day, 0),
			func(time.Time, *time.Time) ([]*pb.OHLC, error) { return []*pb.OHLC{closeBar(day, 10)}, nil })
	}()
	test.waitForLockWaiters(1)

	var mu sync.Mutex
	var calls []syncCall
	fetched := make(chan struct{})
	syncA := make(chan error, 1)
	go func() {
		syncA <- slow.Sync(ctx, "yahoo", "AAPL", day, end,
			func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
				free, err := symbolLockFree(ctx, test.conn, "AAPL")
				if err != nil {
					return nil, err
				}
				mu.Lock()
				calls = append(calls, syncCall{from.Format(time.DateOnly) + ".." + to.Format(time.DateOnly), !free})
				first := len(calls) == 1
				mu.Unlock()
				if first {
					defer close(fetched)
					return aaplBars(aaplPreSplit, from, to), nil
				}
				return aaplBars(aaplAdjusted, from, to), nil
			})
	}()
	select {
	case <-fetched:
	case err := <-syncA:
		test.FailNow("sync A ended before its fetch", "%v", err)
	case <-time.After(10 * time.Second):
		test.FailNow("sync A never fetched")
	}

	// B, on the test's own pool, runs to the end while A waits for its slot.
	test.Require().NoError(test.ohlcStorage.Sync(ctx, "yahoo", "AAPL", bStart, bEnd,
		func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
			return aaplBars(aaplAdjusted, from, to), nil
		}))
	for d, c := range test.aaplCloses() {
		test.InDelta(aaplAdjusted[d], c, 1e-9, "B replaced the days it synced (%s)", d)
	}

	release()
	for name, done := range map[string]chan error{"slot holder": slotHolder, "sync A": syncA} {
		select {
		case err := <-done:
			test.Require().NoError(err, name)
		case <-time.After(10 * time.Second):
			test.FailNow(name + " did not finish")
		}
	}

	got := test.aaplCloses()
	test.Require().Len(got, 3)
	test.InDelta(124.8075, got["2020-08-28"], 1e-9)
	for d, c := range got {
		test.InDelta(aaplAdjusted[d], c, 1e-9, "every row is on the adjusted scale (%s)", d)
	}
	mu.Lock()
	defer mu.Unlock()
	return calls
}
