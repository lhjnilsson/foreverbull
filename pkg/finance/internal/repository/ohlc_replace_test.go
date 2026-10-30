package repository_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func closeBar(t time.Time, c float64) *pb.OHLC {
	return &pb.OHLC{Timestamp: timestamppb.New(t), Open: c, High: c, Low: c, Close: c, Volume: 400}
}

func (test *OHLCTests) storeDays(day time.Time, n int, c float64) {
	for i := 0; i < n; i++ {
		test.Require().NoError(test.ohlcStorage.Store(context.Background(), "yahoo", "ABC", day.AddDate(0, 0, i), c, c, c, c, 100))
	}
}

// The DELETE span is the requested range: stored days after the provider's
// last bar are removed instead of staying on the old scale.
func (test *OHLCTests) TestReplaceRangeDeletesTheRequestedSpan() {
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 4, 500)

	err := test.ohlcStorage.ReplaceRange(context.Background(), "yahoo", "ABC", day, endDay(day, 3),
		[]*pb.OHLC{closeBar(day, 125), closeBar(day.AddDate(0, 0, 1), 125.5)})
	test.Require().NoError(err)
	test.Equal(map[string]float64{"2020-08-26": 125, "2020-08-27": 125.5}, test.storedCloses())
}

// A nil end replaces every stored day from start on.
func (test *OHLCTests) TestReplaceRangeOpenEndDeletesLaterDays() {
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 4, 500)

	err := test.ohlcStorage.ReplaceRange(context.Background(), "yahoo", "ABC", day.AddDate(0, 0, 1), nil,
		[]*pb.OHLC{closeBar(day.AddDate(0, 0, 1), 125)})
	test.Require().NoError(err)
	test.Equal(map[string]float64{"2020-08-26": 500, "2020-08-27": 125}, test.storedCloses())
}

// A refetch that returns no bars fails, names the symbol, and removes nothing.
func (test *OHLCTests) TestReplaceRangeWithNoBarsFails() {
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(day, 2, 500)

	err := test.ohlcStorage.ReplaceRange(context.Background(), "yahoo", "ABC", day, endDay(day, 3), nil)
	test.Require().ErrorIs(err, repository.ErrNoBars)
	test.Require().EqualError(err, "yahoo returned no prices for ABC between 2020-08-26 and 2020-08-29. "+
		"Check that ABC traded between those dates, try again later, or use another provider with --provider.")
	test.Equal(map[string]float64{"2020-08-26": 500, "2020-08-27": 500}, test.storedCloses())
}

// The no-prices sentence names the provider the bars were asked from, in the
// lower-case form backtest create shows.
func (test *OHLCTests) TestReplaceRangeWithNoBarsNamesProvider() {
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	err := test.ohlcStorage.ReplaceRange(context.Background(), "Stooq", "ABC", day, nil, nil)
	test.Require().ErrorIs(err, repository.ErrNoBars)
	test.Require().EqualError(err, "stooq returned no prices for ABC between 2020-08-26 and today. "+
		"Check that ABC traded between those dates, try again later, or use another provider with --provider.")
}

func (test *OHLCTests) countRows() int {
	var n int
	test.Require().NoError(test.conn.QueryRow(context.Background(),
		`SELECT count(*) FROM ohlc WHERE provider = 'yahoo' AND symbol = 'ABC'`).Scan(&n))
	return n
}

// Delete and upsert are keyed on the trading date: a 05:00Z bar replaces the
// 00:00Z row of the same day instead of sitting next to it.
func (test *OHLCTests) TestReplaceRangeKeysOnTradingDate() {
	midnight := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(midnight, 1, 500)

	morning := midnight.Add(5 * time.Hour)
	err := test.ohlcStorage.ReplaceRange(context.Background(), "yahoo", "ABC", morning, &morning,
		[]*pb.OHLC{closeBar(morning, 125)})
	test.Require().NoError(err)
	test.Equal(1, test.countRows())
	test.Equal(map[string]float64{"2020-08-26": 125}, test.storedCloses())
}

// Store (the append path) keys on the trading date too.
func (test *OHLCTests) TestStoreKeysOnTradingDate() {
	midnight := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	test.storeDays(midnight, 1, 500)

	err := test.ohlcStorage.StoreBars(context.Background(), "yahoo", "ABC",
		[]*pb.OHLC{closeBar(midnight.Add(5*time.Hour), 125)})
	test.Require().NoError(err)
	test.Equal(1, test.countRows())
	test.Equal(map[string]float64{"2020-08-26": 125}, test.storedCloses())
}

// Sync takes the per-symbol advisory lock before it reads anything: while
// another transaction holds it, fetch is not called.
func (test *OHLCTests) TestSyncWaitsForTheSymbolLock() {
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)

	holder, err := test.conn.Begin(ctx)
	test.Require().NoError(err)
	defer holder.Rollback(ctx) //nolint:errcheck
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ohlc:' || 'ABC'))`)
	test.Require().NoError(err)

	var fetched atomic.Bool
	done := make(chan error, 1)
	go func() {
		done <- test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, endDay(day, 0),
			func(time.Time, *time.Time) ([]*pb.OHLC, error) {
				fetched.Store(true)
				return []*pb.OHLC{closeBar(day, 125)}, nil
			})
	}()

	select {
	case err := <-done:
		test.FailNow("Sync finished while the lock was held", "%v", err)
	case <-time.After(300 * time.Millisecond):
	}
	test.False(fetched.Load(), "fetch must wait for the lock")

	test.Require().NoError(holder.Rollback(ctx))
	select {
	case err := <-done:
		test.Require().NoError(err)
	case <-time.After(10 * time.Second):
		test.FailNow("Sync did not finish after the lock was released")
	}
	test.True(fetched.Load())
	test.Equal(map[string]float64{"2020-08-26": 125}, test.storedCloses())
}

// Two concurrent syncs of one symbol, one on each scale, end with one row per
// day and every row on one scale.
func (test *OHLCTests) TestConcurrentSyncsLeaveOneScale() {
	ctx := context.Background()
	day := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	end := endDay(day, 3)
	scaled := func(factor float64) repository.FetchBars {
		return func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
			// Widen the window in which both syncs could interleave.
			time.Sleep(50 * time.Millisecond)
			var bars []*pb.OHLC
			for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
				bars = append(bars, closeBar(d, 100*factor))
			}
			return bars, nil
		}
	}

	for round := 0; round < 5; round++ {
		_, err := test.conn.Exec(ctx, `DELETE FROM ohlc`)
		test.Require().NoError(err)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, factor := range []float64{1, 4} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = test.ohlcStorage.Sync(ctx, "yahoo", "ABC", day, end, scaled(factor))
			}()
		}
		wg.Wait()
		test.Require().NoError(errs[0])
		test.Require().NoError(errs[1])

		closes := test.storedCloses()
		test.Equal(4, test.countRows(), "one row per day")
		test.Len(closes, 4)
		scales := map[float64]struct{}{}
		for _, c := range closes {
			scales[c] = struct{}{}
		}
		test.Len(scales, 1, "every stored day on one scale: %v", closes)
	}
}
