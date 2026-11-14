package api_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type SessionTest struct {
	suite.Suite

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.SessionServicerClient
	activity <-chan bool
	// serveErr receives grpc.Server.Serve. The serve goroutine must not
	// call T: Serve can return after the test function does, and logging
	// then races with the testing package.
	serveErr chan error

	mockWorkers *worker.MockPool
	mockTrading *supplier.MockTrading
}

func (test *SessionTest) SetupSuite() {
	test_helper.SetupEnv(test.T())
}

func (test *SessionTest) SetupTest() {
	var err error
	test.serveErr = nil
	test.mockWorkers = new(worker.MockPool)
	test.mockTrading = new(supplier.MockTrading)

	test.listener = bufconn.Listen(1024 * 1024)
	test.server, test.activity, err = api.NewGRPCSessionServer(test.mockTrading)
	test.Require().NoError(err)
	server, listener := test.server, test.listener
	test.serveErr = make(chan error, 1)
	go func() {
		test.serveErr <- server.Serve(listener)
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(test.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return test.listener.Dial()
		}),
	)
	test.Require().NoError(err)
	test.client = servicePb.NewSessionServicerClient(conn)
}

func (test *SessionTest) TearDownTest() {
	if test.server != nil {
		test.server.Stop()
	}
	if test.serveErr != nil {
		err := <-test.serveErr
		test.serveErr = nil
		if err != nil {
			test.T().Logf("error serving server: %v", err)
		}
	}
	if test.listener != nil {
		if err := test.listener.Close(); err != nil {
			test.T().Errorf("Error closing listener: %v", err)
		}
	}
}

func TestSession(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (test *SessionTest) TestPlaceOrdersReturnsPlacedOrder() {
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "AAPL" && order.GetBrokerOrderId() == ""
	})).Return(&pb.Order{
		Symbol:        "AAPL",
		Amount:        10,
		Status:        pb.OrderStatus_ORDER_STATUS_ACCEPTED,
		BrokerOrderId: "brk-aapl",
	}, nil).Once()
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "MSFT" && order.GetBrokerOrderId() == ""
	})).Return(&pb.Order{
		Symbol:        "MSFT",
		Amount:        -4,
		Status:        pb.OrderStatus_ORDER_STATUS_FILLED,
		BrokerOrderId: "brk-msft",
	}, nil).Once()

	rsp, err := test.client.PlaceOrders(context.Background(), &servicePb.PlaceOrderRequest{
		Orders: []*pb.Order{
			{Symbol: "AAPL", Amount: 10},
			{Symbol: "MSFT", Amount: -4},
		},
	})
	test.Require().NoError(err)
	test.Require().Len(rsp.GetOrders(), 2)
	test.Equal(pb.OrderStatus_ORDER_STATUS_ACCEPTED, rsp.GetOrders()[0].GetStatus())
	test.Equal("brk-aapl", rsp.GetOrders()[0].GetBrokerOrderId())
	test.Equal("AAPL", rsp.GetOrders()[0].GetSymbol())
	test.Equal(int32(10), rsp.GetOrders()[0].GetAmount())
	test.Equal(pb.OrderStatus_ORDER_STATUS_FILLED, rsp.GetOrders()[1].GetStatus())
	test.Equal("brk-msft", rsp.GetOrders()[1].GetBrokerOrderId())
	test.Equal("MSFT", rsp.GetOrders()[1].GetSymbol())
	test.Equal(int32(-4), rsp.GetOrders()[1].GetAmount())
	test.mockTrading.AssertExpectations(test.T())
}

func (test *SessionTest) TestPlaceOrdersNamesEveryFailure() {
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "AAPL" && order.GetAmount() == 10
	})).Return(&pb.Order{
		Symbol:        "AAPL",
		Amount:        10,
		Status:        pb.OrderStatus_ORDER_STATUS_ACCEPTED,
		BrokerOrderId: "brk-aapl",
	}, nil).Once()
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "MSFT" && order.GetAmount() == 5
	})).Return(nil, fmt.Errorf("isin US5949181045: %w", supplier.ErrNotTradable)).Once()
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "KO" && order.GetAmount() == -2
	})).Return(nil, fmt.Errorf("amount is zero: %w", supplier.ErrRejected)).Once()

	rsp, err := test.client.PlaceOrders(context.Background(), &servicePb.PlaceOrderRequest{
		Orders: []*pb.Order{
			{Symbol: "AAPL", Amount: 10},
			{Symbol: "MSFT", Isin: "US5949181045", Amount: 5},
			{Symbol: "KO", Amount: -2},
		},
	})
	test.Require().Error(err)
	test.Require().Nil(rsp)
	st, ok := status.FromError(err)
	test.Require().True(ok)
	test.Equal(codes.FailedPrecondition, st.Code())
	test.Require().ErrorContains(err, "place orders failed")
	test.Require().ErrorContains(err, "placed order 1 AAPL amount 10 broker brk-aapl")
	test.Require().ErrorContains(err, "order 2 MSFT US5949181045 amount 5")
	test.Require().ErrorContains(err, "instrument is not tradable")
	test.Require().ErrorContains(err, "order 3 KO amount -2")
	test.Require().ErrorContains(err, "amount is zero")
	test.Require().ErrorContains(err, "order rejected")

	orders := ordersFromPlaceError(test, err)
	test.Require().Len(orders, 3)
	test.Equal(pb.OrderStatus_ORDER_STATUS_ACCEPTED, orders[0].GetStatus())
	test.Equal("brk-aapl", orders[0].GetBrokerOrderId())
	test.Equal("AAPL", orders[0].GetSymbol())
	test.Equal(int32(10), orders[0].GetAmount())
	test.Equal(pb.OrderStatus_ORDER_STATUS_REJECTED, orders[1].GetStatus())
	test.Equal("MSFT", orders[1].GetSymbol())
	test.Equal("US5949181045", orders[1].GetIsin())
	test.Equal(int32(5), orders[1].GetAmount())
	test.Empty(orders[1].GetBrokerOrderId())
	test.Contains(orders[1].GetRejectReason(), "instrument is not tradable")
	test.Equal(pb.OrderStatus_ORDER_STATUS_REJECTED, orders[2].GetStatus())
	test.Equal("KO", orders[2].GetSymbol())
	test.Equal(int32(-2), orders[2].GetAmount())
	test.Contains(orders[2].GetRejectReason(), "amount is zero")
	test.Contains(orders[2].GetRejectReason(), "order rejected")
	test.mockTrading.AssertNumberOfCalls(test.T(), "PlaceOrder", 3)
	test.mockTrading.AssertExpectations(test.T())
}

func (test *SessionTest) TestPlaceOrdersUnavailableStillNamesEveryOrder() {
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "AAPL"
	})).Return(nil, fmt.Errorf("alpaca http 500: %w", supplier.ErrUnavailable)).Once()
	test.mockTrading.On("PlaceOrder", mock.Anything, mock.MatchedBy(func(order *pb.Order) bool {
		return order.GetSymbol() == "MSFT"
	})).Return(&pb.Order{
		Symbol:        "MSFT",
		Amount:        -4,
		Status:        pb.OrderStatus_ORDER_STATUS_FILLED,
		BrokerOrderId: "brk-msft",
	}, nil).Once()

	rsp, err := test.client.PlaceOrders(context.Background(), &servicePb.PlaceOrderRequest{
		Orders: []*pb.Order{
			{Symbol: "AAPL", Amount: 10},
			{Symbol: "MSFT", Amount: -4},
		},
	})
	test.Require().Error(err)
	test.Require().Nil(rsp)
	st, ok := status.FromError(err)
	test.Require().True(ok)
	test.Equal(codes.Unavailable, st.Code())
	test.Require().ErrorContains(err, "order 1 AAPL amount 10")
	test.Require().ErrorContains(err, "trading backend unavailable")
	test.Require().ErrorContains(err, "placed order 2 MSFT amount -4 broker brk-msft")

	orders := ordersFromPlaceError(test, err)
	test.Require().Len(orders, 2)
	test.Equal(pb.OrderStatus_ORDER_STATUS_REJECTED, orders[0].GetStatus())
	test.Contains(orders[0].GetRejectReason(), "trading backend unavailable")
	test.Equal(pb.OrderStatus_ORDER_STATUS_FILLED, orders[1].GetStatus())
	test.Equal("brk-msft", orders[1].GetBrokerOrderId())
	test.mockTrading.AssertNumberOfCalls(test.T(), "PlaceOrder", 2)
}

// ordersFromPlaceError reads the PlaceOrderResponse carried on the status.
// The unary reply is nil when the call fails.
func ordersFromPlaceError(test *SessionTest, err error) []*pb.Order {
	test.T().Helper()
	st, ok := status.FromError(err)
	test.Require().True(ok)
	for _, detail := range st.Details() {
		body, ok := detail.(*servicePb.PlaceOrderResponse)
		if ok {
			return body.GetOrders()
		}
	}
	test.FailNow("PlaceOrderResponse missing from status details", err)
	return nil
}
