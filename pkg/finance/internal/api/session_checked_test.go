package api

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckedSessionSymbolsKeepsTheBenchmark(t *testing.T) {
	require.Equal(t, []string{"AAPL", "SPY"}, checkedSessionSymbols([]string{"AAPL"}, "SPY"))
	trading := supplier.NewMockTrading(t)
	trading.On("GetPortfolio", mock.Anything).Return(&pb.Portfolio{}, nil)
	pool := new(worker.MockPool)
	pool.On("Process", mock.Anything, mock.Anything, []string{"SPY"}, mock.Anything).Return([]*pb.Order{}, nil)
	gs := &grpcSessionServer{trading: trading, wp: pool}
	WithCheckedSymbols(checkedSessionSymbols([]string{"AAPL"}, "SPY"))(gs)
	_, err := gs.Run(context.Background(), &servicePb.RunRequest{Symbols: []string{"SPY"}})
	require.NoError(t, err)
}

func checkedSession(symbols ...string) *grpcSessionServer {
	gs := &grpcSessionServer{}
	WithCheckedSymbols(symbols)(gs)
	return gs
}

func TestRunRejectsASymbolOutsideTheCheckedSet(t *testing.T) {
	trading := supplier.NewMockTrading(t)
	gs := checkedSession("AAPL")
	gs.trading = trading
	_, err := gs.Run(context.Background(), &servicePb.RunRequest{Symbols: []string{"VOD.L"}})
	require.EqualError(t, err, "rpc error: code = FailedPrecondition desc = VOD.L wasn't checked when this trading session started, so it can't be traded. No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes VOD.L.")
	trading.AssertNotCalled(t, "GetPortfolio", mock.Anything)
}

func TestRunAllowsACheckedSymbol(t *testing.T) {
	trading := supplier.NewMockTrading(t)
	trading.On("GetPortfolio", mock.Anything).Return(&pb.Portfolio{Cash: 1}, nil)
	pool := new(worker.MockPool)
	pool.On("Process", mock.Anything, mock.Anything, []string{"AAPL"}, mock.Anything).
		Return([]*pb.Order{}, nil)
	gs := checkedSession("AAPL")
	gs.trading = trading
	gs.wp = pool
	_, err := gs.Run(context.Background(), &servicePb.RunRequest{Symbols: []string{"AAPL"}})
	require.NoError(t, err)
	pool.AssertCalled(t, "Process", mock.Anything, mock.Anything, []string{"AAPL"}, mock.Anything)
}

func TestPlaceOrdersRejectsASymbolOutsideTheCheckedSet(t *testing.T) {
	trading := supplier.NewMockTrading(t)
	gs := checkedSession("AAPL")
	gs.trading = trading
	_, err := gs.PlaceOrders(context.Background(), &servicePb.PlaceOrderRequest{
		Orders: []*pb.Order{{Symbol: "AAPL", Amount: 1}, {Symbol: "VOD.L", Amount: 2}},
	})
	require.EqualError(t, err, "rpc error: code = FailedPrecondition desc = VOD.L wasn't checked when this trading session started, so it can't be traded. No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes VOD.L.")
	trading.AssertNotCalled(t, "PlaceOrder", mock.Anything, mock.Anything)
}

func TestPlaceOrdersAllowsACheckedSymbol(t *testing.T) {
	trading := supplier.NewMockTrading(t)
	trading.On("PlaceOrder", mock.Anything, mock.Anything).Return(&pb.Order{
		Symbol: "AAPL", Amount: 1, Status: pb.OrderStatus_ORDER_STATUS_ACCEPTED,
	}, nil)
	gs := checkedSession("AAPL")
	gs.trading = trading
	rsp, err := gs.PlaceOrders(context.Background(), &servicePb.PlaceOrderRequest{
		Orders: []*pb.Order{{Symbol: "AAPL", Amount: 1}},
	})
	require.NoError(t, err)
	require.Equal(t, "AAPL", rsp.GetOrders()[0].GetSymbol())
	trading.AssertCalled(t, "PlaceOrder", mock.Anything, mock.Anything)
}

func TestRunCheckedSetIsComparedWithoutCase(t *testing.T) {
	trading := supplier.NewMockTrading(t)
	trading.On("GetPortfolio", mock.Anything).Return(&pb.Portfolio{}, nil)
	pool := new(worker.MockPool)
	pool.On("Process", mock.Anything, mock.AnythingOfType("time.Time"), []string{"aapl"}, mock.Anything).
		Return([]*pb.Order{}, nil)
	gs := checkedSession("AAPL")
	gs.trading = trading
	gs.wp = pool
	_, err := gs.Run(context.Background(), &servicePb.RunRequest{Symbols: []string{"aapl"}})
	require.NoError(t, err)
}
