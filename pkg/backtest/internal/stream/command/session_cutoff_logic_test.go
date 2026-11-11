package command

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/require"
)

func TestBacktestThroughCutoffStopsTheBundleAtTheRecordedEnd(t *testing.T) {
	t.Parallel()
	stored := &backtest.Backtest{
		Name: "demo",
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 2},
			EndDate:   &pb.Date{Year: 2024, Month: 3, Day: 28},
		}},
	}
	closed := time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC)
	capped, err := backtestThroughCutoff(stored, closed)
	require.NoError(t, err)
	require.Equal(t, int32(28), stored.GetPeriods()[0].GetEndDate().GetDay())
	require.Equal(t, int32(27), capped.GetPeriods()[0].GetEndDate().GetDay())

	_, err = backtestThroughCutoff(stored, time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC))
	require.EqualError(t, err, "the recorded session cut-off 2024-03-29 is after the backtest ends on 2024-03-28")

	later := &backtest.Backtest{
		Periods: []*backtest.Backtest_Period{{
			StartDate: &pb.Date{Year: 2024, Month: 3, Day: 28},
			EndDate:   &pb.Date{Year: 2024, Month: 3, Day: 28},
		}},
	}
	_, err = backtestThroughCutoff(later, closed)
	require.EqualError(t, err, "a period starts on 2024-03-28, after the session cut-off 2024-03-27")
}
