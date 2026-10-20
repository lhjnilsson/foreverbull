package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/require"
)

func day(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	require.NoError(t, err)
	return parsed
}

func dayPtr(t *testing.T, value string) *time.Time {
	t.Helper()
	parsed := day(t, value)
	return &parsed
}

// Membership is half-open [from, to); the run period is the closed [start, end].
func TestUniverseEmptyBetween(t *testing.T) {
	const isin = "US0378331005"
	start := day(t, "2023-01-02")
	end := day(t, "2023-01-31")

	cases := []struct {
		name      string
		intervals []repository.MemberInterval
		empty     bool
	}{
		{
			name:      "membership starts after the run ends",
			intervals: []repository.MemberInterval{{ISIN: isin, FromDate: day(t, "2023-02-28")}},
			empty:     true,
		},
		{
			name:      "membership starts the day after the run ends",
			intervals: []repository.MemberInterval{{ISIN: isin, FromDate: day(t, "2023-02-01")}},
			empty:     true,
		},
		{
			name: "membership ends on the run start, which to excludes",
			intervals: []repository.MemberInterval{
				{ISIN: isin, FromDate: day(t, "2022-11-30"), ToDate: dayPtr(t, "2023-01-02")},
			},
			empty: true,
		},
		{
			name: "every interval falls outside the run",
			intervals: []repository.MemberInterval{
				{ISIN: isin, FromDate: day(t, "2022-10-31"), ToDate: dayPtr(t, "2022-12-30")},
				{ISIN: "US5949181045", FromDate: day(t, "2023-02-28")},
			},
			empty: true,
		},
		{
			name: "a degenerate interval inside the run covers no date",
			intervals: []repository.MemberInterval{
				{ISIN: isin, FromDate: day(t, "2023-01-10"), ToDate: dayPtr(t, "2023-01-10")},
			},
			empty: true,
		},
		{
			name:      "membership starts on the last run date",
			intervals: []repository.MemberInterval{{ISIN: isin, FromDate: day(t, "2023-01-31")}},
			empty:     false,
		},
		{
			name: "membership ends the day after the run start",
			intervals: []repository.MemberInterval{
				{ISIN: isin, FromDate: day(t, "2022-11-30"), ToDate: dayPtr(t, "2023-01-03")},
			},
			empty: false,
		},
		{
			name: "membership only for part of the run is not empty",
			intervals: []repository.MemberInterval{
				{ISIN: isin, FromDate: day(t, "2023-01-15"), ToDate: dayPtr(t, "2023-01-20")},
			},
			empty: false,
		},
		{
			name:      "open membership that started before the run",
			intervals: []repository.MemberInterval{{ISIN: isin, FromDate: day(t, "2020-01-31")}},
			empty:     false,
		},
		{
			name: "one covering interval among outside ones",
			intervals: []repository.MemberInterval{
				{ISIN: isin, FromDate: day(t, "2023-03-31")},
				{ISIN: "US5949181045", FromDate: day(t, "2022-12-30"), ToDate: dayPtr(t, "2023-01-05")},
			},
			empty: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.empty, universeEmptyBetween(tc.intervals, start, end))
		})
	}
}

func TestUniverseEmptyBetweenRefusesAReversedRunPeriod(t *testing.T) {
	intervals := []repository.MemberInterval{{ISIN: "US0378331005", FromDate: day(t, "2024-01-31")}}
	require.False(t, universeEmptyBetween(intervals, day(t, "2023-01-31"), day(t, "2023-01-02")))
}

type fakeMembers struct {
	intervals []repository.MemberInterval
	err       error
	asked     string
}

func (f *fakeMembers) List(_ context.Context, backtestName string) ([]repository.MemberInterval, error) {
	f.asked = backtestName
	return f.intervals, f.err
}

func runOfJanuary(name string) *backtest.Execution {
	return &backtest.Execution{
		Backtest:  name,
		StartDate: &pb.Date{Year: 2023, Month: 1, Day: 2},
		EndDate:   &pb.Date{Year: 2023, Month: 1, Day: 31},
	}
}

func TestExecutionUniverse(t *testing.T) {
	later := []repository.MemberInterval{{ISIN: "US0378331005", FromDate: day(t, "2023-02-28")}}
	covering := []repository.MemberInterval{{ISIN: "US0378331005", FromDate: day(t, "2022-12-30")}}

	t.Run("symbol-list backtest has no membership rows and is never empty", func(t *testing.T) {
		members := &fakeMembers{intervals: []repository.MemberInterval{}}
		source, empty, err := executionUniverse(context.Background(), members, runOfJanuary("demo"))
		require.NoError(t, err)
		require.Equal(t, "demo", members.asked)
		require.Equal(t, backtest.UniverseSource_UNIVERSE_SOURCE_SYMBOL_LIST, source)
		require.False(t, empty)
	})
	t.Run("ETF backtest with no member during the run is empty", func(t *testing.T) {
		source, empty, err := executionUniverse(context.Background(), &fakeMembers{intervals: later}, runOfJanuary("etf"))
		require.NoError(t, err)
		require.Equal(t, backtest.UniverseSource_UNIVERSE_SOURCE_ETF, source)
		require.True(t, empty)
	})
	t.Run("ETF backtest with a member during the run is not empty", func(t *testing.T) {
		source, empty, err := executionUniverse(context.Background(), &fakeMembers{intervals: covering}, runOfJanuary("etf"))
		require.NoError(t, err)
		require.Equal(t, backtest.UniverseSource_UNIVERSE_SOURCE_ETF, source)
		require.False(t, empty)
	})
	t.Run("ETF run without an end date does not claim an empty universe", func(t *testing.T) {
		execution := runOfJanuary("etf")
		execution.EndDate = nil
		source, empty, err := executionUniverse(context.Background(), &fakeMembers{intervals: later}, execution)
		require.NoError(t, err)
		require.Equal(t, backtest.UniverseSource_UNIVERSE_SOURCE_ETF, source)
		require.False(t, empty)
	})
	t.Run("a failed membership read fails the call", func(t *testing.T) {
		readErr := errors.New("connection refused")
		source, empty, err := executionUniverse(context.Background(), &fakeMembers{err: readErr}, runOfJanuary("etf"))
		require.ErrorIs(t, err, readErr)
		require.Equal(t, backtest.UniverseSource_UNIVERSE_SOURCE_UNSPECIFIED, source)
		require.False(t, empty)
	})
}
