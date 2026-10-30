package repository_test

import (
	"context"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// A day missing in the middle of the stored range is asked for again on the
// next incremental fetch and filled. The other stored rows keep their prices,
// even when the provider now returns different ones for them.
func (test *OHLCTests) TestSyncFillsAGapInsideTheStoredRange() {
	ctx := context.Background()
	mon := time.Date(2020, 8, 24, 0, 0, 0, 0, time.UTC) // Monday
	test.storeDays(mon, 2, 500)                         // Mon 08-24, Tue 08-25
	test.storeDays(mon.AddDate(0, 0, 3), 2, 500)        // Thu 08-27, Fri 08-28; Wed 08-26 is missing

	var spans [][2]string
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", mon, endDay(mon, 8), // through Tue 09-01
		func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
			spans = append(spans, [2]string{from.Format("2006-01-02"), to.Format("2006-01-02")})
			return []*pb.OHLC{
				closeBar(mon.AddDate(0, 0, 2), 501), // the gap
				closeBar(mon.AddDate(0, 0, 3), 450), // stored, must stay 500
				closeBar(mon.AddDate(0, 0, 4), 500), // overlap day
				closeBar(mon.AddDate(0, 0, 7), 502), // Mon 08-31
				closeBar(mon.AddDate(0, 0, 8), 503), // Tue 09-01
			}, nil
		})
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-08-26", "2020-09-01"}}, spans,
		"one fetch, from the first missing weekday")
	test.Equal(map[string]float64{
		"2020-08-24": 500,
		"2020-08-25": 500,
		"2020-08-26": 501,
		"2020-08-27": 500,
		"2020-08-28": 500,
		"2020-08-31": 502,
		"2020-09-01": 503,
	}, test.storedCloses())
}

// Weekends are not gaps: with every weekday stored the fetch still begins on
// the last stored day.
func (test *OHLCTests) TestSyncSkipsWeekendsWhenLookingForGaps() {
	ctx := context.Background()
	thu := time.Date(2020, 8, 27, 0, 0, 0, 0, time.UTC)
	test.storeDays(thu, 2, 500)                  // Thu, Fri
	test.storeDays(thu.AddDate(0, 0, 4), 1, 500) // Mon 08-31

	var froms []string
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", thu, endDay(thu, 5),
		func(from time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			froms = append(froms, from.Format("2006-01-02"))
			return []*pb.OHLC{closeBar(thu.AddDate(0, 0, 4), 500), closeBar(thu.AddDate(0, 0, 5), 501)}, nil
		})
	test.Require().NoError(err)
	test.Equal([]string{"2020-08-31"}, froms)
	test.Equal(map[string]float64{
		"2020-08-27": 500, "2020-08-28": 500, "2020-08-31": 500, "2020-09-01": 501,
	}, test.storedCloses())
}

// A gap fill that finds the overlap day moved still replaces the whole stored
// history, so a split can't leave the filled day on another scale.
func (test *OHLCTests) TestSyncGapFillStillReplacesAfterASplit() {
	ctx := context.Background()
	mon := time.Date(2020, 8, 24, 0, 0, 0, 0, time.UTC)
	test.storeDays(mon, 1, 500)
	test.storeDays(mon.AddDate(0, 0, 2), 1, 500) // Tue 08-25 missing

	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", mon, endDay(mon, 3),
		func(from time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			bars := []*pb.OHLC{}
			for d := from; !d.After(mon.AddDate(0, 0, 3)); d = d.AddDate(0, 0, 1) {
				bars = append(bars, closeBar(d, 125))
			}
			return bars, nil
		})
	test.Require().NoError(err)
	test.Equal(map[string]float64{
		"2020-08-24": 125, "2020-08-25": 125, "2020-08-26": 125, "2020-08-27": 125,
	}, test.storedCloses())
}
