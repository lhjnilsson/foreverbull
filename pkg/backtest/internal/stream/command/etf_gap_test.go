package command

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	"github.com/stretchr/testify/require"
)

const gapISIN = "US0378331005"

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

func (s gapScraper) ListSnapshotDates(string, time.Time, time.Time) ([]time.Time, error) {
	return s.dates, nil
}

func (s gapScraper) GetSnapshot(_ string, asOf time.Time) ([]dependency.Asset, error) {
	return s.byDay[asOf.Format("2006-01-02")], nil
}

func TestLoadETFSnapshotsFillsLeadingGapFromLaterListing(t *testing.T) {
	jan := time.Date(2020, time.January, 31, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC)
	later := []dependency.Asset{{
		Symbol: "AAPL", Name: "Apple Inc.", Sector: "Technology", AssetType: "Equity", ISIN: gapISIN,
	}}
	scraper := gapScraper{
		dates: []time.Time{jan, feb},
		byDay: map[string][]dependency.Asset{
			"2020-02-29": later,
		},
	}
	start := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC)

	snaps, err := loadETFSnapshots(scraper, "iShares S&P 100", start, end)
	require.NoError(t, err)
	// The start is seeded from the first listing, which a leading gap filled from February.
	require.Len(t, snaps, 3)
	require.Equal(t, start, snaps[0].AsOf)
	require.Equal(t, jan, snaps[1].AsOf)
	require.Equal(t, []universe.Holding{{
		ISIN: gapISIN, Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity",
	}}, snaps[0].Holdings)
	require.Equal(t, snaps[0].Holdings, snaps[1].Holdings)
	require.Equal(t, snaps[2].Holdings, snaps[1].Holdings)

	result := universe.Diff(snaps)
	require.Equal(t, []universe.Member{{
		ISIN: gapISIN, FromDate: start, ToDate: nil,
	}}, result.Members)
	require.Equal(t, []universe.Alias{{
		ISIN: gapISIN, Symbol: "AAPL", FromDate: start, ToDate: nil,
	}}, result.Aliases)
}

// The first as-of opens membership at the backtest start, so no day before the
// first month-end is empty. A start that is itself an as-of is not doubled.
func TestLoadETFSnapshotsSeedsBacktestStart(t *testing.T) {
	jan := time.Date(2020, time.January, 31, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC)
	holdings := []dependency.Asset{{Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity", ISIN: gapISIN}}
	scraper := gapScraper{
		dates: []time.Time{jan, feb},
		byDay: map[string][]dependency.Asset{"2020-01-31": holdings, "2020-02-29": holdings},
	}
	cases := []struct {
		name      string
		start     time.Time
		snapCount int
	}{
		{name: "year start", start: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), snapCount: 3},
		{name: "mid month", start: time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC), snapCount: 3},
		{name: "month end", start: jan, snapCount: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snaps, err := loadETFSnapshots(scraper, "iShares S&P 100", tc.start, feb)
			require.NoError(t, err)
			require.Len(t, snaps, tc.snapCount)
			require.Equal(t, tc.start, snaps[0].AsOf)

			result := universe.Diff(snaps)
			for day := tc.start; !day.After(feb); day = day.AddDate(0, 0, 1) {
				require.Equal(t, []string{gapISIN}, universe.MembersAt(result.Members, day), day.Format("2006-01-02"))
			}
		})
	}
}

func TestLoadETFSnapshotsNoHoldingsNamesWindow(t *testing.T) {
	jan := time.Date(2020, time.January, 31, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC)
	scraper := gapScraper{dates: []time.Time{jan, feb}}
	_, err := loadETFSnapshots(scraper, "iShares S&P 100",
		time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.February, 29, 0, 0, 0, 0, time.UTC))
	require.EqualError(t, err, "iShares has no holdings file for iShares S&P 100 between 2020-01-01 and 2020-02-29")
	require.NotContains(t, err.Error(), "invalid character")
}
