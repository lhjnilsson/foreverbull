package api

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/require"
)

func TestDividendWindowUsesEnabledSymbolsAndTheBenchmark(t *testing.T) {
	t.Parallel()
	item := &backtest.Backtest{
		Name:      "spy",
		Benchmark: " SPY ",
		Provider:  "yahoo",
		Periods: []*backtest.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2024, Month: 3, Day: 1},
				EndDate:   &pb.Date{Year: 2024, Month: 6, Day: 28},
				Assets: []*backtest.Backtest_Period_Asset{
					{Symbol: "KO", Enabled: true},
					{Symbol: "PEP", Enabled: false},
					{Symbol: " KO ", Enabled: true},
				},
			},
			{
				StartDate: &pb.Date{Year: 2024, Month: 1, Day: 2},
				EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
				Assets: []*backtest.Backtest_Period_Asset{
					{Symbol: "TSM", Enabled: true},
				},
			},
		},
	}
	symbols, start, end, ok := dividendWindow(item)
	require.True(t, ok)
	require.Equal(t, []string{"KO", "SPY", "TSM"}, symbols)
	require.Equal(t, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), end)
}

func TestDividendWindowIsEmptyWithoutASpanOrASymbol(t *testing.T) {
	t.Parallel()
	_, _, _, ok := dividendWindow(nil)
	require.False(t, ok)
	_, _, _, ok = dividendWindow(&backtest.Backtest{Name: "spy", Provider: "yahoo"})
	require.False(t, ok)
	_, _, _, ok = dividendWindow(&backtest.Backtest{
		Name: "spy", Provider: "yahoo",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 2},
			EndDate:   &pb.Date{Year: 2024, Month: 6, Day: 28},
			Assets:    []*backtest.Backtest_Period_Asset{{Symbol: "KO", Enabled: false}},
		}},
	})
	require.False(t, ok)
}
