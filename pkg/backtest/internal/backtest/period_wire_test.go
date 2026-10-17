package backtest

import (
	"os"
	"testing"

	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The fixture is proto.Marshal output. Regenerating it from Python would not prove Go can read the bytes.
func TestPythonPeriodPayload(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../../client/tests/testdata/backtest_period.bin")
	require.NoError(t, err)

	got := &pb.Period{}
	require.NoError(t, proto.Unmarshal(raw, got))

	sharpe := 1.25
	want := &pb.Period{
		Date:           &commonpb.Date{Year: 2024, Month: 1, Day: 2},
		PNL:            12.5,
		PortfolioValue: 10050.25,
		Sharpe:         &sharpe,
		Positions: []*finance.Position{
			{
				Symbol:        "AAPL",
				Amount:        10,
				CostBasis:     150.5,
				LastSalePrice: 155,
				LastSaleDate:  &timestamppb.Timestamp{Seconds: 1704229200},
				Isin:          "US0378331005",
			},
			{Symbol: "IBM", Amount: -4},
		},
	}
	require.True(t, proto.Equal(want, got))
	require.Nil(t, got.Alpha)
	require.NotNil(t, got.Sharpe)
	require.InDelta(t, sharpe, got.GetSharpe(), 1e-9)
	require.Nil(t, got.GetPositions()[1].LastSaleDate)
	require.Equal(t, int64(1704229200), got.GetPositions()[0].GetLastSaleDate().GetSeconds())
}
