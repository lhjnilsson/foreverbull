package stream_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/require"
)

func TestCreateBacktestFromETFOrchestration(t *testing.T) {
	t.Parallel()

	o, err := stream.CreateBacktestFromETF("sp100_backtest", "iShares S&P 100", "2022-01-01", "2023-01-01", "", "")
	require.NoError(t, err)
	require.Len(t, o.Steps, 3)
	require.Equal(t, int32(1), o.Steps[0].Num)
	require.Equal(t, int32(2), o.Steps[1].Num)
	require.Equal(t, int32(3), o.Steps[2].Num)
}

func TestCreateBacktestFromBacktestOrchestration(t *testing.T) {
	t.Parallel()

	o, err := stream.CreateBacktestFromBacktest(&backtestpb.Backtest{
		Name:      "test_backtest_from_backtest",
		Benchmark: "OEF",
		Periods: []*backtestpb.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
				Assets: []*backtestpb.Backtest_Period_Asset{
					{
						Symbol:  "AAPL",
						Enabled: true,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, o.Steps, 3)
	require.Equal(t, int32(1), o.Steps[0].Num)
	require.Equal(t, int32(2), o.Steps[1].Num)
	require.Equal(t, int32(3), o.Steps[2].Num)
}
