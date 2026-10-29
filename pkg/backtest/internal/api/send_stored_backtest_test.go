package api

import (
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSendStoredBacktestHidesLoadError(t *testing.T) {
	rsp := &servicePb.CreateBacktestResponse{
		INGESTING: &pb.Step{State: pb.State_STATE_COMPLETED},
		STORING:   &pb.Step{State: pb.State_STATE_UNSPECIFIED},
	}
	load := func() (*backtest.Backtest, error) {
		return nil, errors.New(`ERROR: relation "backtest" does not exist (SQLSTATE 42P01)`)
	}

	err := sendStoredBacktest(rsp, nil, "demo", load)

	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "Backtest demo was created, but its details couldn't be loaded. Run `fbull backtest get demo` to see it.", status.Convert(err).Message())
	require.NotContains(t, err.Error(), "SQLSTATE")
}
