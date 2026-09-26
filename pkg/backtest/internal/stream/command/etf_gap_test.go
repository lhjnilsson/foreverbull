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
	require.Len(t, snaps, 2)
	require.Equal(t, jan, snaps[0].AsOf)
	require.Equal(t, []universe.Holding{{
		ISIN: gapISIN, Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity",
	}}, snaps[0].Holdings)
	require.Equal(t, snaps[1].Holdings, snaps[0].Holdings)

	result := universe.Diff(snaps)
	require.Equal(t, []universe.Member{{
		ISIN: gapISIN, FromDate: jan, ToDate: nil,
	}}, result.Members)
	require.Equal(t, []universe.Alias{{
		ISIN: gapISIN, Symbol: "AAPL", FromDate: jan, ToDate: nil,
	}}, result.Aliases)
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
