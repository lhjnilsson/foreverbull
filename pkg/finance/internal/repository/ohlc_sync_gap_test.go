package repository_test

import (
	"context"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// gapDay is a 2020-08 day in UTC.
func gapDay(d int) time.Time { return time.Date(2020, 8, d, 0, 0, 0, 0, time.UTC) }

// storeCloses stores yahoo ABC bars with the given close per day.
func (test *OHLCTests) storeCloses(closes map[string]float64) {
	for day, c := range closes {
		t, err := time.Parse("2006-01-02", day)
		test.Require().NoError(err)
		test.Require().NoError(test.ohlcStorage.Store(context.Background(), "yahoo", "ABC", t, c, c, c, c, 100))
	}
}

// fetchCloses is a fetch that returns a bar for every day in closes from
// from through to, and records each span it was asked for.
func fetchCloses(closes map[string]float64, spans *[][2]string) func(time.Time, *time.Time) ([]*pb.OHLC, error) {
	return func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
		*spans = append(*spans, [2]string{from.Format("2006-01-02"), to.Format("2006-01-02")})
		bars := []*pb.OHLC{}
		for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
			if c, ok := closes[d.Format("2006-01-02")]; ok {
				bars = append(bars, closeBar(d, c))
			}
		}
		return bars, nil
	}
}

// AAPL around its 2020-08-31 4:1 split, as Yahoo returned it before the
// split (old scale) and after it (adjusted, old / 4).
var (
	aaplOldScale = map[string]float64{
		"2020-08-24": 503.43, "2020-08-25": 499.30, "2020-08-26": 506.09, "2020-08-27": 500.04, "2020-08-28": 499.23,
	}
	aaplNewScale = map[string]float64{
		"2020-08-24": 125.8575, "2020-08-25": 124.825, "2020-08-26": 126.5225, "2020-08-27": 125.01, "2020-08-28": 124.8075,
	}
)

// storedAAPLWithoutAug26 stores the adjusted AAPL closes with 2020-08-26
// missing.
func (test *OHLCTests) storedAAPLWithoutAug26() {
	stored := map[string]float64{}
	for day, c := range aaplNewScale {
		if day != "2020-08-26" {
			stored[day] = c
		}
	}
	test.storeCloses(stored)
}

// A day missing in the middle of the stored range is asked for again on the
// next incremental fetch and filled. The fetch begins on the stored day
// before the gap, and the other stored rows keep their prices even when the
// provider now returns different ones for them.
func (test *OHLCTests) TestSyncFillsAGapInsideTheStoredRange() {
	ctx := context.Background()
	mon := gapDay(24)
	test.storeDays(mon, 2, 500)                  // Mon 08-24, Tue 08-25
	test.storeDays(mon.AddDate(0, 0, 3), 2, 500) // Thu 08-27, Fri 08-28; Wed 08-26 is missing

	var spans [][2]string
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", mon, endDay(mon, 8), // through Tue 09-01
		func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
			spans = append(spans, [2]string{from.Format("2006-01-02"), to.Format("2006-01-02")})
			return []*pb.OHLC{
				closeBar(mon.AddDate(0, 0, 1), 500), // stored day before the gap
				closeBar(mon.AddDate(0, 0, 2), 501), // the gap
				closeBar(mon.AddDate(0, 0, 3), 450), // stored, must stay 500
				closeBar(mon.AddDate(0, 0, 4), 500), // last stored day
				closeBar(mon.AddDate(0, 0, 7), 502), // Mon 08-31
				closeBar(mon.AddDate(0, 0, 8), 503), // Tue 09-01
			}, nil
		})
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-08-25", "2020-09-01"}}, spans,
		"one fetch, from the stored day before the first missing weekday")
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

// storeWeekdays stores close c for every weekday from from through to except
// the skipped days.
func (test *OHLCTests) storeWeekdays(from, to time.Time, c float64, skip ...string) {
	skipped := map[string]bool{}
	for _, s := range skip {
		skipped[s] = true
	}
	closes := map[string]float64{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || skipped[d.Format("2006-01-02")] {
			continue
		}
		closes[d.Format("2006-01-02")] = c
	}
	test.storeCloses(closes)
}

// Only the requested range, and no more than gapScanDays before the last
// stored day, is looked at for gaps. 2020-07-03 (the observed Independence
// Day holiday, which no provider ever fills) is the only missing weekday of
// the stored 2020 history. A sync for August fetches from the last stored
// day, right before the August start, and so does a sync for the whole year
// as ingest sends it: neither reaches back to July 3, which stays missing.
func (test *OHLCTests) TestSyncLooksForGapsOnlyInsideTheRequestedRange() {
	ctx := context.Background()
	test.storeWeekdays(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2020, 7, 31, 0, 0, 0, 0, time.UTC),
		300, "2020-07-03")

	provider := map[string]float64{"2020-07-03": 300, "2020-07-31": 300}
	for d := gapDay(3); !d.After(gapDay(31)); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			provider[d.Format("2006-01-02")] = 301
		}
	}
	var spans [][2]string
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", gapDay(3), endDay(gapDay(3), 28), fetchCloses(provider, &spans))
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-07-31", "2020-08-31"}}, spans)

	// The whole year, the way ingest asks every run, from the same stored
	// history (August removed again): 07-03 is before the gap scan window
	// (07-17 on), so it is still not fetched.
	_, err = test.conn.Exec(ctx, `DELETE FROM ohlc WHERE time >= '2020-08-01'`)
	test.Require().NoError(err)
	spans = nil
	err = test.ohlcStorage.Sync(ctx, "yahoo", "ABC", time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), endDay(gapDay(31), 0),
		fetchCloses(provider, &spans))
	test.Require().NoError(err)
	test.Equal([][2]string{{"2020-07-31", "2020-08-31"}}, spans)
	got := test.storedCloses()
	test.NotContains(got, "2020-07-03", "a gap before the requested range is left as it is")
	test.InDelta(300, got["2020-07-31"], 1e-9)
	test.InDelta(301, got["2020-08-03"], 1e-9)
	test.InDelta(301, got["2020-08-31"], 1e-9)
}

// A gap on the first requested day has no stored day before it inside the
// range, so the fetch begins on the gap itself (here the August start) and
// the first stored day after it is checked, together with the last one.
// 2020-07-03 before the range still stays missing.
func (test *OHLCTests) TestSyncGapOnTheFirstRequestedDayFetchesFromIt() {
	ctx := context.Background()
	test.storeWeekdays(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), gapDay(14), 300, "2020-07-03", "2020-08-03")

	provider := map[string]float64{"2020-07-03": 300}
	for d := gapDay(3); !d.After(gapDay(31)); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			provider[d.Format("2006-01-02")] = 300
		}
	}
	provider["2020-08-03"] = 299
	var spans [][2]string
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", gapDay(3), endDay(gapDay(3), 28), fetchCloses(provider, &spans))
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-08-03", "2020-08-31"}}, spans)
	got := test.storedCloses()
	test.NotContains(got, "2020-07-03")
	test.InDelta(299, got["2020-08-03"], 1e-9)
	test.InDelta(300, got["2020-08-31"], 1e-9)
}

// Weekends are not gaps: with every weekday stored the fetch still begins on
// the last stored day.
func (test *OHLCTests) TestSyncSkipsWeekendsWhenLookingForGaps() {
	ctx := context.Background()
	thu := gapDay(27)
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

// A gap fill that finds the check day moved still replaces the whole stored
// history, so a split can't leave the filled day on another scale.
func (test *OHLCTests) TestSyncGapFillStillReplacesAfterASplit() {
	ctx := context.Background()
	mon := gapDay(24)
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

// The AAPL split fixture: stored history on the adjusted scale with
// 2020-08-26 missing, and a provider still on the old scale. Whether the gap
// has a stored day before it in the range (checked: 08-25) or not (the gap
// is the first requested day, so the first stored day after it, 08-27, is
// checked), the old-scale close doesn't match, so no old-scale bar is put
// next to the adjusted rows: the whole history is fetched again and
// replaced, and every stored day ends up on one scale.
func (test *OHLCTests) TestSyncRefusesAGapFillOnAnotherScale() {
	for _, tc := range []struct {
		name  string
		start time.Time
		spans [][2]string
	}{
		{"stored day before the gap", gapDay(24), [][2]string{{"2020-08-25", "2020-08-31"}, {"2020-08-24", "2020-08-31"}}},
		{"gap on the first requested day", gapDay(26), [][2]string{{"2020-08-26", "2020-08-31"}, {"2020-08-24", "2020-08-31"}}},
	} {
		test.Run(tc.name, func() {
			_, err := test.conn.Exec(context.Background(), `DELETE FROM ohlc`)
			test.Require().NoError(err)
			test.storedAAPLWithoutAug26()

			var spans [][2]string
			err = test.ohlcStorage.Sync(context.Background(), "yahoo", "ABC", tc.start, endDay(gapDay(31), 0),
				fetchCloses(aaplOldScale, &spans))
			test.Require().NoError(err)

			test.Equal(tc.spans, spans, "the gap fetch, then the full refetch")
			test.Equal(aaplOldScale, test.storedCloses(), "every day on the provider's scale, none mixed")
		})
	}
}

// Only the check day next to the gap moved: the stored day before the gap
// (08-25) comes back more than rescaleTolerance off while the last stored day
// (08-28) still matches. That alone must replace the whole history, so the
// filled 08-26 can't be put next to rows on another scale.
func (test *OHLCTests) TestSyncGapFillReplacesWhenOnlyTheDayBeforeTheGapMoved() {
	test.storedAAPLWithoutAug26()

	provider := map[string]float64{
		"2020-08-24": 125.8575, "2020-08-25": 126.10, "2020-08-26": 126.5225,
		"2020-08-27": 125.01, "2020-08-28": 124.8075, "2020-08-31": 129.04,
	}
	var spans [][2]string
	err := test.ohlcStorage.Sync(context.Background(), "yahoo", "ABC", gapDay(24), endDay(gapDay(31), 0),
		fetchCloses(provider, &spans))
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-08-25", "2020-08-31"}, {"2020-08-24", "2020-08-31"}}, spans,
		"the gap fetch, then the full refetch")
	test.Equal(provider, test.storedCloses())
}

// A gap fill never rewrites a stored row before the last stored day: the
// provider sends 08-27 slightly off (within rescaleTolerance), the checks
// pass, 08-26 is filled and 08-27 stays 125.01. The last stored day (08-28)
// is the overlap day and is upserted within tolerance, as an incremental
// fetch always did.
func (test *OHLCTests) TestSyncGapFillNeverRewritesAStoredRow() {
	test.storedAAPLWithoutAug26()

	provider := map[string]float64{
		"2020-08-24": 125.8575, "2020-08-25": 124.825, "2020-08-26": 126.5225,
		"2020-08-27": 125.05, "2020-08-28": 124.80, "2020-08-31": 129.04,
	}
	var spans [][2]string
	err := test.ohlcStorage.Sync(context.Background(), "yahoo", "ABC", gapDay(24), endDay(gapDay(31), 0),
		fetchCloses(provider, &spans))
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-08-25", "2020-08-31"}}, spans)
	got := test.storedCloses()
	test.InDelta(125.01, got["2020-08-27"], 1e-9)
	test.InDelta(124.80, got["2020-08-28"], 1e-9)
	test.InDelta(126.5225, got["2020-08-26"], 1e-9)
	test.InDelta(129.04, got["2020-08-31"], 1e-9)
}

// nyseHolidays is the NYSE's weekday holidays in 2020 and on 2021-01-01.
var nyseHolidays = map[string]bool{
	"2020-01-01": true, "2020-01-20": true, "2020-02-17": true, "2020-04-10": true, "2020-05-25": true,
	"2020-07-03": true, "2020-09-07": true, "2020-11-26": true, "2020-12-25": true, "2021-01-01": true,
}

func isTradingDay(d time.Time) bool {
	return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday && !nyseHolidays[d.Format(time.DateOnly)]
}

// tradingBars is close c for every NYSE trading day from through to, as a
// real exchange returns it: no bar on a holiday.
func tradingBars(from time.Time, to *time.Time, c float64) []*pb.OHLC {
	out := []*pb.OHLC{}
	for d := from; !d.After(*to); d = d.AddDate(0, 0, 1) {
		if isTradingDay(d) {
			out = append(out, closeBar(d, c))
		}
	}
	return out
}

// A fully stored 2020 (every trading day, so only the holidays are missing
// weekdays) and two daily syncs with the backtest's whole range, as ingest
// runs them. Neither fetch begins before gapScanDays before the last stored
// day: the holidays from 2020-01-20 on never widen it, only Christmas inside
// the window does. Before the window a sync fetched from 2020-01-20, 485
// bars instead of a handful.
func (test *OHLCTests) TestSyncGapScanIgnoresGapsOlderThanWindow() {
	ctx := context.Background()
	start := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	stored := tradingBars(start, endDay(time.Date(2020, 12, 30, 0, 0, 0, 0, time.UTC), 0), 100)
	test.Require().NoError(test.ohlcStorage.StoreBars(ctx, "yahoo", "ABC", stored))

	jan4 := time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC)
	var spans []string
	barsReturned := 0
	for _, end := range []time.Time{time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC), jan4} {
		_, last, _, err := test.ohlcStorage.Range(ctx, "yahoo", "ABC")
		test.Require().NoError(err)
		windowStart := last.AddDate(0, 0, -14)
		e := end
		test.Require().NoError(test.ohlcStorage.Sync(ctx, "yahoo", "ABC", start, &e,
			func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
				test.False(from.Before(windowStart), "fetch from %s is before the window start %s",
					from.Format(time.DateOnly), windowStart.Format(time.DateOnly))
				spans = append(spans, from.Format(time.DateOnly)+".."+to.Format(time.DateOnly))
				b := tradingBars(from, to, 100)
				barsReturned += len(b)
				return b, nil
			}))
	}
	test.Equal([]string{"2020-12-24..2020-12-31", "2020-12-24..2021-01-04"}, spans)
	test.Equal(11, barsReturned, "12-24, 12-28..12-31, then 12-24, 12-28..12-31 and 01-04")
	got := test.storedCloses()
	test.NotContains(got, "2020-01-20")
	test.Contains(got, "2021-01-04")
	test.NotContains(got, "2021-01-01", "no bar on a holiday")
}

// storeJune2020Except stores close 300 for every weekday from 2020-06-01
// through Mon 2020-06-29 except missing.
func (test *OHLCTests) storeJune2020Except(missing string) {
	test.storeWeekdays(time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 6, 29, 0, 0, 0, 0, time.UTC),
		300, missing)
}

// june2020Provider has close 300 for every weekday of June 2020 through
// 2020-07-02.
func june2020Provider() map[string]float64 {
	closes := map[string]float64{}
	for d := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC); !d.After(time.Date(2020, 7, 2, 0, 0, 0, 0, time.UTC)); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			closes[d.Format(time.DateOnly)] = 300
		}
	}
	return closes
}

// A real gap 10 days before the newest stored bar (Fri 2020-06-19, newest
// Mon 06-29) is inside the window and is filled, even when the sync asks for
// the whole range from 06-01.
func (test *OHLCTests) TestSyncFillsAGapInsideTheWindow() {
	test.storeJune2020Except("2020-06-19")
	provider := june2020Provider()
	provider["2020-06-19"] = 301

	var spans [][2]string
	err := test.ohlcStorage.Sync(context.Background(), "yahoo", "ABC", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC),
		endDay(time.Date(2020, 7, 2, 0, 0, 0, 0, time.UTC), 0), fetchCloses(provider, &spans))
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-06-18", "2020-07-02"}}, spans)
	test.InDelta(301, test.storedCloses()["2020-06-19"], 1e-9)
}

// A gap 20 days before the newest stored bar (Tue 2020-06-09, newest Mon
// 06-29) is outside the window: it is left alone, and nothing before the
// window start (06-15) is fetched.
func (test *OHLCTests) TestSyncLeavesAGapOutsideTheWindow() {
	test.storeJune2020Except("2020-06-09")

	var spans [][2]string
	err := test.ohlcStorage.Sync(context.Background(), "yahoo", "ABC", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC),
		endDay(time.Date(2020, 7, 2, 0, 0, 0, 0, time.UTC), 0), fetchCloses(june2020Provider(), &spans))
	test.Require().NoError(err)

	test.Equal([][2]string{{"2020-06-29", "2020-07-02"}}, spans)
	for _, span := range spans {
		test.GreaterOrEqual(span[0], "2020-06-15", "nothing before the window start is fetched")
	}
	got := test.storedCloses()
	test.NotContains(got, "2020-06-09")
	test.Contains(got, "2020-07-02")
}

// Another sync fills the gap between the fetch before the lock and the
// locked decision: the locked decision fetches again from the last stored
// day, and the row the other sync stored is kept.
func (test *OHLCTests) TestSyncKeepsAGapFilledByAnotherSyncBeforeTheLock() {
	ctx := context.Background()
	mon := gapDay(24)
	test.storeDays(mon, 1, 500)
	test.storeDays(mon.AddDate(0, 0, 2), 1, 500) // 08-25 missing
	var froms []string
	err := test.ohlcStorage.Sync(ctx, "yahoo", "ABC", mon, endDay(mon, 3),
		func(from time.Time, _ *time.Time) ([]*pb.OHLC, error) {
			froms = append(froms, from.Format(time.DateOnly))
			if len(froms) == 1 { // the other sync lands now
				test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "ABC", mon.AddDate(0, 0, 1), 499, 499, 499, 499, 1))
			}
			return []*pb.OHLC{
				closeBar(mon, 500), closeBar(mon.AddDate(0, 0, 1), 501),
				closeBar(mon.AddDate(0, 0, 2), 500), closeBar(mon.AddDate(0, 0, 3), 502),
			}, nil
		})
	test.Require().NoError(err)
	test.Equal([]string{"2020-08-24", "2020-08-26"}, froms)
	test.Equal(map[string]float64{"2020-08-24": 500, "2020-08-25": 499, "2020-08-26": 500, "2020-08-27": 502},
		test.storedCloses())
}
