package api_test

import (
	"context"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type SessionTest struct {
	suite.Suite

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.SessionServicerClient
	activity <-chan bool

	mockWorkers *worker.MockPool
	mockTrading *supplier.MockTrading
}

func (test *SessionTest) SetupSuite() {
	test_helper.SetupEnv(test.T())
}

func (test *SessionTest) SetupTest() {
	var err error
	test.mockWorkers = new(worker.MockPool)
	test.mockTrading = new(supplier.MockTrading)

	test.listener = bufconn.Listen(1024 * 1024)
	test.server, test.activity, err = api.NewGRPCSessionServer(test.mockTrading)
	test.Require().NoError(err)
	go func() {
		if err := test.server.Serve(test.listener); err != nil {
			test.T().Logf("error serving server: %v", err)
		}
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
	err := test.listener.Close()
	if err != nil {
		test.T().Errorf("Error closing listener: %v", err)
	}
	test.server.Stop()
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
