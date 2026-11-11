package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

const (
	gapISIN  = "US0378331005"
	msftISIN = "US5949181045"
	gapETF   = "iShares S&P 100"
)

// gapScraper serves fixed as-of dates. ListSnapshotDates returns the ones in
// the requested range, as the iShares scraper does, so the seed walk before
// the start sees only the as-of dates before the start.
type gapScraper struct {
	dates []time.Time
	byDay map[string][]dependency.Asset
}

func (s gapScraper) GetAssets(string, ...string) (map[string][]dependency.Asset, error) {
	return nil, nil
}

func (s gapScraper) GetBenchmark(string) (string, error) {
	return "", nil
}

func (s gapScraper) ListSnapshotDates(_ string, start, end time.Time) ([]time.Time, error) {
	dates := make([]time.Time, 0, len(s.dates))
	for _, asOf := range s.dates {
		if asOf.Before(start) || asOf.After(end) {
			continue
		}
		dates = append(dates, asOf)
	}
	return dates, nil
}

func (s gapScraper) GetSnapshot(_ string, asOf time.Time) ([]dependency.Asset, error) {
	return s.byDay[asOf.Format("2006-01-02")], nil
}

func day(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", raw)
	require.NoError(t, err)
	return parsed
}

// captureLog swaps the global logger for one that writes JSON lines to a buffer.
// Tests that call it do not run in parallel.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = previous })
	return &buf
}

var (
	aaplAsset = dependency.Asset{Symbol: "AAPL", Name: "Apple Inc.", Sector: "Technology", AssetType: "Equity", ISIN: gapISIN}
	msftAsset = dependency.Asset{Symbol: "MSFT", Name: "Microsoft Corp.", Sector: "Technology", AssetType: "Equity", ISIN: msftISIN}
)

// The start holds the last list published on or before it: 2019-12-31 for a
// 2020-01-01 or 2020-01-15 start, and the start itself when it is an as-of.
// That list is held from the start until the next as-of; no day is empty.
func TestLoadETFSnapshotsSeedsFromLastListingOnOrBeforeStart(t *testing.T) {
	dec := day(t, "2019-12-31")
	jan := day(t, "2020-01-31")
	feb := day(t, "2020-02-29")
	scraper := gapScraper{
		dates: []time.Time{dec, jan, feb},
		byDay: map[string][]dependency.Asset{
			"2019-12-31": {aaplAsset},
			"2020-01-31": {aaplAsset, msftAsset},
			"2020-02-29": {aaplAsset, msftAsset},
		},
	}
	cases := []struct {
		name      string
		start     string
		snapCount int
		atStart   []string
	}{
		{name: "year start", start: "2020-01-01", snapCount: 3, atStart: []string{gapISIN}},
		{name: "mid month", start: "2020-01-15", snapCount: 3, atStart: []string{gapISIN}},
		{name: "month end", start: "2020-01-31", snapCount: 2, atStart: []string{gapISIN, msftISIN}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLog(t)
			start := day(t, tc.start)
			snaps, err := loadETFSnapshots(scraper, gapETF, start, feb)
			require.NoError(t, err)
			require.Len(t, snaps, tc.snapCount)
			require.Equal(t, start, snaps[0].AsOf)

			result := universe.Diff(snaps)
			require.Equal(t, tc.atStart, universe.MembersAt(result.Members, start))
			for d := start; !d.After(feb); d = d.AddDate(0, 0, 1) {
				require.NotEmpty(t, universe.MembersAt(result.Members, d), d.Format("2006-01-02"))
			}
			require.Empty(t, logs.String(), "a list on or before the start is not a fallback")
		})
	}
}

// A stock that joins in January is not a member before the as-of that first
// lists it, for a 2020-01-01 start. With only month-ends that is 2020-01-31;
// with a mid-January as-of (a cached file) it is that day. It is a member from
// the as-of on, and the 2019-12-31 list holds until then.
func TestLoadETFSnapshotsLaterAddIsNotAMemberBeforeItsAsOf(t *testing.T) {
	start := day(t, "2020-01-01")
	end := day(t, "2020-02-29")
	cases := []struct {
		name  string
		dates []string
		added string
	}{
		{name: "first in the month-end list", dates: []string{"2019-12-31", "2020-01-31", "2020-02-29"}, added: "2020-01-31"},
		{name: "added in a mid-January list", dates: []string{"2019-12-31", "2020-01-15", "2020-01-31", "2020-02-29"}, added: "2020-01-15"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := gapScraper{byDay: map[string][]dependency.Asset{}}
			for _, raw := range tc.dates {
				scraper.dates = append(scraper.dates, day(t, raw))
				scraper.byDay[raw] = []dependency.Asset{aaplAsset, msftAsset}
			}
			scraper.byDay["2019-12-31"] = []dependency.Asset{aaplAsset}

			snaps, err := loadETFSnapshots(scraper, gapETF, start, end)
			require.NoError(t, err)
			result := universe.Diff(snaps)

			added := day(t, tc.added)
			require.Contains(t, result.Members, universe.Member{ISIN: msftISIN, FromDate: added, ToDate: nil})
			require.Contains(t, result.Members, universe.Member{ISIN: gapISIN, FromDate: start, ToDate: nil})
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				at := universe.MembersAt(result.Members, d)
				if d.Before(added) {
					require.Equal(t, []string{gapISIN}, at, "%s is before the as-of that adds MSFT", d.Format("2006-01-02"))
				} else {
					require.Equal(t, []string{gapISIN, msftISIN}, at, d.Format("2006-01-02"))
				}
			}
		})
	}
}

// A month-end with no holdings file before the start does not stop the walk:
// the 2019-11-30 list seeds a 2020-01-01 start when every month in the window
// has its own list. December is before the start, so it is not a gap.
func TestLoadETFSnapshotsWalksBackPastUnpublishedMonthEnd(t *testing.T) {
	logs := captureLog(t)
	start := day(t, "2020-01-01")
	jan := day(t, "2020-01-31")
	end := day(t, "2020-02-29")
	scraper := gapScraper{
		dates: []time.Time{day(t, "2019-11-30"), day(t, "2019-12-31"), jan, end},
		byDay: map[string][]dependency.Asset{
			"2019-11-30": {aaplAsset},
			"2020-01-31": {aaplAsset, msftAsset},
			"2020-02-29": {aaplAsset, msftAsset},
		},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, start, end)
	require.NoError(t, err)
	require.Len(t, snaps, 3)
	require.Equal(t, start, snaps[0].AsOf)
	require.Equal(t, []universe.Holding{{
		ISIN: gapISIN, Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity",
	}}, snaps[0].Holdings)

	result := universe.Diff(snaps)
	require.Equal(t, []universe.Member{
		{ISIN: gapISIN, FromDate: start, ToDate: nil},
		{ISIN: msftISIN, FromDate: jan, ToDate: nil},
	}, result.Members)
	require.Equal(t, []string{gapISIN}, universe.MembersAt(result.Members, start))
	require.Equal(t, []string{gapISIN, msftISIN}, universe.MembersAt(result.Members, jan))
	require.Empty(t, logs.String())
}

// Nothing was published on or before the start. Create fails and names the
// ETF, the start, and the first list after it. The later list is not copied
// onto the start.
func TestLoadETFSnapshotsFailsWhenNothingPublishedOnOrBeforeStart(t *testing.T) {
	jan := day(t, "2020-01-31")
	feb := day(t, "2020-02-29")
	scraper := gapScraper{
		dates: []time.Time{jan, feb},
		byDay: map[string][]dependency.Asset{
			"2020-01-31": {aaplAsset},
			"2020-02-29": {aaplAsset, msftAsset},
		},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-15"), feb)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list on or before 2020-01-15. "+
		"The first list was published on 2020-01-31. Choose a start on or after 2020-01-31.")
}

// January has no file and the first list is February. Moving the start to
// February drops January, so the error is only the start sentence.
func TestLoadETFSnapshotsFailsWhenLeadingGapWouldUseALaterList(t *testing.T) {
	jan := day(t, "2020-01-31")
	feb := day(t, "2020-02-29")
	scraper := gapScraper{
		dates: []time.Time{jan, feb},
		byDay: map[string][]dependency.Asset{"2020-02-29": {aaplAsset}},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), feb)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list on or before 2020-01-01. "+
		"The first list was published on 2020-02-29. Choose a start on or after 2020-02-29.")
}

// January is before the first list, so a later start drops it. March is still
// missing after that list and is the only month the gap sentence names.
func TestLoadETFSnapshotsFailsWhenStartAndALaterMonthAreMissing(t *testing.T) {
	end := day(t, "2020-03-31")
	scraper := gapScraper{
		dates: []time.Time{day(t, "2020-01-31"), day(t, "2020-02-29"), end},
		byDay: map[string][]dependency.Asset{"2020-02-29": {aaplAsset}},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), end)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list on or before 2020-01-01. "+
		"The first list was published on 2020-02-29. Choose a start on or after 2020-02-29. "+
		"iShares S&P 100 has no holdings list for 2020-03-31. Choose dates that don't include 2020-03-31.")
}

// January and February are both before the first list, so neither is named
// again. The start is still not taken from March.
func TestLoadETFSnapshotsFailsWhenLeadingGapIsSeveralMonths(t *testing.T) {
	mar := day(t, "2020-03-31")
	scraper := gapScraper{
		dates: []time.Time{day(t, "2020-01-31"), day(t, "2020-02-29"), mar},
		byDay: map[string][]dependency.Asset{"2020-03-31": {aaplAsset}},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), mar)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list on or before 2020-01-01. "+
		"The first list was published on 2020-03-31. Choose a start on or after 2020-03-31.")
}

// December was published and January was not. The old path copied December
// into January and create succeeded. Create now fails and names that month.
func TestLoadETFSnapshotsFailsWhenGapWouldUseAnEarlierList(t *testing.T) {
	end := day(t, "2020-02-29")
	scraper := gapScraper{
		dates: []time.Time{day(t, "2019-12-31"), day(t, "2020-01-31"), end},
		byDay: map[string][]dependency.Asset{
			"2019-12-31": {aaplAsset},
			"2020-02-29": {aaplAsset, msftAsset},
		},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), end)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list for 2020-01-31. "+
		"Choose dates that don't include 2020-01-31.")
}

// January and February are unpublished between two published lists. They are
// one gap, not two copies of December.
func TestLoadETFSnapshotsFailsWhenEarlierListWouldFillAGap(t *testing.T) {
	end := day(t, "2020-03-31")
	scraper := gapScraper{
		dates: []time.Time{day(t, "2019-12-31"), day(t, "2020-01-31"), day(t, "2020-02-29"), end},
		byDay: map[string][]dependency.Asset{
			"2019-12-31": {aaplAsset},
			"2020-03-31": {aaplAsset, msftAsset},
		},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), end)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list for 2020-01-31 to 2020-02-29. "+
		"Choose dates that don't include 2020-01-31 to 2020-02-29.")
}

// January and March are missing and February is published, so the months are
// named separately. An earlier list does not fill either one.
func TestLoadETFSnapshotsFailsWhenTwoMonthsWouldUseAnEarlierList(t *testing.T) {
	end := day(t, "2020-03-31")
	scraper := gapScraper{
		dates: []time.Time{day(t, "2019-12-31"), day(t, "2020-01-31"), day(t, "2020-02-29"), end},
		byDay: map[string][]dependency.Asset{
			"2019-12-31": {aaplAsset},
			"2020-02-29": {aaplAsset, msftAsset},
		},
	}
	snaps, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), end)
	require.Nil(t, snaps)
	require.EqualError(t, err, "iShares S&P 100 has no holdings list for 2020-01-31 and 2020-03-31. "+
		"Choose dates that don't include 2020-01-31 or 2020-03-31.")
}

func TestLoadETFSnapshotsNoHoldingsNamesWindow(t *testing.T) {
	scraper := gapScraper{dates: []time.Time{day(t, "2020-01-31"), day(t, "2020-02-29")}}
	_, err := loadETFSnapshots(scraper, gapETF, day(t, "2020-01-01"), day(t, "2020-02-29"))
	require.EqualError(t, err, "iShares S&P 100 has no holdings list between 2020-01-01 and 2020-02-29. "+
		"Choose dates that include a published list.")
	require.NotContains(t, err.Error(), "invalid character")
}
