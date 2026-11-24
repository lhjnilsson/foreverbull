package usercopy_test

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/stretchr/testify/require"
)

func day(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, raw)
	require.NoError(t, err)
	return parsed
}

func TestETFNoHoldingsInWindow(t *testing.T) {
	got := usercopy.ETFNoHoldingsInWindow("iShares S&P 100", day(t, "2020-01-01"), day(t, "2020-02-29"))
	require.Equal(t, "iShares S&P 100 has no holdings list between 2020-01-01 and 2020-02-29. "+
		"Choose dates that include a published list.", got)
}

func TestETFNoHoldingsAtStart(t *testing.T) {
	got := usercopy.ETFNoHoldingsAtStart("iShares S&P 100", day(t, "2020-01-15"), day(t, "2020-01-31"))
	require.Equal(t, "iShares S&P 100 has no holdings list on or before 2020-01-15. "+
		"The first list was published on 2020-01-31. Choose a start on or after 2020-01-31.", got)
}

func TestHoldingsListUnusable(t *testing.T) {
	day := day(t, "2022-12-31")
	mixed := []usercopy.HoldingsFault{
		{Row: 2, Label: "ABC", Kind: usercopy.HoldingsNoISIN},
		{Row: 4, Label: "Broken Row", Kind: usercopy.HoldingsUnread},
		{Column: "sectorName", Kind: usercopy.HoldingsUnread},
	}
	require.Equal(t, "The iShares S&P 100 holdings list for 2022-12-31 can't be used: "+
		"row 2 (ABC) has no ISIN; row 4 (Broken Row) couldn't be read; the sectorName column couldn't be read. "+
		"Choose dates that don't include 2022-12-31. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
		usercopy.HoldingsListUnusable("iShares S&P 100", day, mixed))

	noISIN := []usercopy.HoldingsFault{{Row: 1, Label: "ABC", Kind: usercopy.HoldingsNoISIN}}
	require.Equal(t, "The iShares S&P 100 holdings list for 2022-12-31 can't be used: "+
		"row 1 (ABC) has no ISIN. Choose dates that don't include 2022-12-31.",
		usercopy.HoldingsListUnusable("iShares S&P 100", day, noISIN))

	column := []usercopy.HoldingsFault{{Column: "ticker", Kind: usercopy.HoldingsUnread}}
	require.Equal(t, "The iShares S&P 100 holdings list for 2022-12-31 can't be used: "+
		"the ticker column couldn't be read. Choose dates that don't include 2022-12-31. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
		usercopy.HoldingsListUnusable("iShares S&P 100", day, column))

	require.Equal(t, "The iShares S&P 100 holdings list can't be used: "+
		"the ticker column couldn't be read. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
		usercopy.HoldingsListUnusable("iShares S&P 100", time.Time{}, column))

	require.Equal(t, "The iShares holdings list for 2022-12-31 can't be used: "+
		"row 1 (ABC) has no ISIN. Choose dates that don't include 2022-12-31.",
		usercopy.HoldingsListUnusable("", day, noISIN))
	require.Equal(t, "The iShares holdings list can't be used: row 1 (ABC) has no ISIN. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
		usercopy.HoldingsListUnusable("  ", time.Time{}, noISIN))
	require.Equal(t, "The iShares S&P 100 holdings list can't be used: row 1 (ABC) has no ISIN. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
		usercopy.HoldingsListUnusable("iShares S&P 100", time.Time{}, noISIN))
}

func TestETFHoldingsGapOneMonth(t *testing.T) {
	got := usercopy.ETFHoldingsGap("iShares S&P 100", []time.Time{day(t, "2020-01-31")})
	require.Equal(t, "iShares S&P 100 has no holdings list for 2020-01-31. "+
		"Choose dates that don't include 2020-01-31.", got)
}

func TestETFHoldingsGapSeveralMonths(t *testing.T) {
	got := usercopy.ETFHoldingsGap("iShares S&P 100", []time.Time{
		day(t, "2020-01-31"), day(t, "2020-02-29"), day(t, "2020-03-31"),
	})
	require.Equal(t, "iShares S&P 100 has no holdings list for 2020-01-31 to 2020-03-31. "+
		"Choose dates that don't include 2020-01-31 to 2020-03-31.", got)
}

func TestETFHoldingsGapSeparateMonths(t *testing.T) {
	got := usercopy.ETFHoldingsGap("iShares S&P 100", []time.Time{
		day(t, "2020-03-31"), day(t, "2020-01-31"),
	})
	require.Equal(t, "iShares S&P 100 has no holdings list for 2020-01-31 and 2020-03-31. "+
		"Choose dates that don't include 2020-01-31 or 2020-03-31.", got)
}

func TestETFHoldingsGapThreeSeparateMonths(t *testing.T) {
	got := usercopy.ETFHoldingsGap("iShares S&P 100", []time.Time{
		day(t, "2020-05-31"), day(t, "2020-01-31"), day(t, "2020-03-31"),
	})
	require.Equal(t, "iShares S&P 100 has no holdings list for 2020-01-31, 2020-03-31 and 2020-05-31. "+
		"Choose dates that don't include 2020-01-31, 2020-03-31 or 2020-05-31.", got)
}

func TestETFHoldingsGapRangeAndMonth(t *testing.T) {
	got := usercopy.ETFHoldingsGap("iShares S&P 100", []time.Time{
		day(t, "2020-04-30"), day(t, "2020-01-31"), day(t, "2020-02-29"),
	})
	require.Equal(t, "iShares S&P 100 has no holdings list for 2020-01-31 to 2020-02-29 and 2020-04-30. "+
		"Choose dates that don't include 2020-01-31 to 2020-02-29 or 2020-04-30.", got)
}
