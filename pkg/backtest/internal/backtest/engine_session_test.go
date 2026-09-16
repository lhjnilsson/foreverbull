package backtest

import (
	"context"
	"testing"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeMemberSymbols struct {
	byDate map[string][]string
	err    error
}

func (f fakeMemberSymbols) SymbolsAt(_ context.Context, _ string, t time.Time) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byDate[t.UTC().Format("2006-01-02")], nil
}

func drain(ch <-chan *servicePb.RunExecutionResponse) {
	for range ch {
	}
}

func periodAt(t time.Time) *servicePb.GetCurrentPeriodResponse {
	return &servicePb.GetCurrentPeriodResponse{
		IsRunning: true,
		Portfolio: &finance.Portfolio{Timestamp: timestamppb.New(t)},
	}
}

func TestRunBacktestProcessReceivesMembersAtT(t *testing.T) {
	t.Parallel()

	fbDay := time.Date(2022, 12, 31, 15, 0, 0, 0, time.UTC)
	metaDay := time.Date(2023, 1, 31, 15, 0, 0, 0, time.UTC)

	client := servicePb.NewMockEngineSessionClient(t)
	pool := worker.NewMockPool(t)
	client.On("RunBacktest", mock.Anything, mock.Anything).
		Return(&servicePb.RunBacktestResponse{NumPeriods: 2}, nil).Once()
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(periodAt(fbDay), nil).Once()
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(periodAt(metaDay), nil).Once()
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(&servicePb.GetCurrentPeriodResponse{IsRunning: false}, nil).Once()
	client.On("PlaceOrdersAndContinue", mock.Anything, mock.Anything).
		Return(&servicePb.PlaceOrdersAndContinueResponse{}, nil).Twice()

	pool.On("Process", mock.Anything, fbDay, []string{"AAPL", "FB"}, mock.Anything).
		Return(nil, nil).Once()
	pool.On("Process", mock.Anything, metaDay, []string{"AAPL", "META", "MSFT"}, mock.Anything).
		Return(nil, nil).Once()

	session := &ZipelineSession{
		client: client,
		symbolsAt: fakeMemberSymbols{byDate: map[string][]string{
			"2022-12-31": {"AAPL", "FB"},
			"2023-01-31": {"AAPL", "META", "MSFT"},
		}},
	}

	ch, err := session.RunBacktest(context.Background(), &pb.Execution{Backtest: "sp100"}, pool)
	require.NoError(t, err)
	drain(ch)
}

func TestRunBacktestEmptyUniversePassesEmptySymbols(t *testing.T) {
	t.Parallel()

	day := time.Date(2022, 12, 30, 15, 0, 0, 0, time.UTC)
	client := servicePb.NewMockEngineSessionClient(t)
	pool := worker.NewMockPool(t)
	client.On("RunBacktest", mock.Anything, mock.Anything).
		Return(&servicePb.RunBacktestResponse{NumPeriods: 1}, nil).Once()
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(periodAt(day), nil).Once()
	client.On("GetCurrentPeriod", mock.Anything, mock.Anything).
		Return(&servicePb.GetCurrentPeriodResponse{IsRunning: false}, nil).Once()
	client.On("PlaceOrdersAndContinue", mock.Anything, mock.Anything).
		Return(&servicePb.PlaceOrdersAndContinueResponse{}, nil).Once()
	pool.On("Process", mock.Anything, day, []string{}, mock.Anything).
		Return(nil, nil).Once()

	session := &ZipelineSession{
		client:    client,
		symbolsAt: fakeMemberSymbols{byDate: map[string][]string{}},
	}

	ch, err := session.RunBacktest(context.Background(), &pb.Execution{Backtest: "sp100"}, pool)
	require.NoError(t, err)
	drain(ch)
}

func TestMemberSymbolsAtEmptyInputs(t *testing.T) {
	t.Parallel()

	got, err := memberSymbolsAt(context.Background(), nil, "sp100", time.Now())
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = memberSymbolsAt(context.Background(), fakeMemberSymbols{}, "", time.Now())
	require.NoError(t, err)
	require.Empty(t, got)
}
