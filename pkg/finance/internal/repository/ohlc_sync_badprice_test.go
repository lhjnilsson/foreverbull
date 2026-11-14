package repository_test

import (
	"context"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// A check day whose stored row is not a bar is not an overlap match. Sync
// replaces the history with the provider's bars instead of failing. The
// prices are inserted with SQL so Store's check does not refuse them.
func (test *OHLCTests) TestSyncReplacesWhenTheCheckDayIsNotABar() {
	friday := gapDay(21)
	monday := gapDay(24)
	cases := []struct {
		name                   string
		open, high, low, close string
	}{
		{name: "zero close", open: "10", high: "10", low: "10", close: "0"},
		{name: "NaN close", open: "10", high: "10", low: "10", close: "NaN"},
		{name: "zero open", open: "0", high: "10", low: "10", close: "10"},
		{name: "negative high", open: "10", high: "-1", low: "-1", close: "10"},
		{name: "negative low", open: "10", high: "10", low: "-1", close: "10"},
	}
	for _, tc := range cases {
		test.Run(tc.name, func() {
			ctx := context.Background()
			_, err := test.conn.Exec(ctx, `DELETE FROM ohlc`)
			test.Require().NoError(err)
			test.insertStoredBar("ABC", friday, "1", "2", "0.5", "500")
			test.insertStoredBar("ABC", monday, tc.open, tc.high, tc.low, tc.close)

			err = test.ohlcStorage.Sync(ctx, "yahoo", "ABC", friday, endDay(monday, 2),
				func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
					bars := []*pb.OHLC{}
					for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
						if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
							continue
						}
						bars = append(bars, closeBar(d, 125))
					}
					return bars, nil
				})
			test.Require().NoError(err)
			test.Equal(map[string]float64{
				"2020-08-21": 125,
				"2020-08-24": 125,
				"2020-08-25": 125,
				"2020-08-26": 125,
			}, test.storedCloses())
		})
	}
}
