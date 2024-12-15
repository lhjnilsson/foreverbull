package servicer

import (
	"context"
	"errors"
	"log"
	"net"
	"testing"

	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type TradingServerTest struct {
	suite.Suite

	pgx     *pgxpool.Pool
	trading *supplier.MockTrading

	listener *bufconn.Listener
	server   *grpc.Server
	client   pb.TradingClient
}

func TestTradingServerTest(t *testing.T) {
	suite.Run(t, new(TradingServerTest))
}

func (suite *TradingServerTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (suite *TradingServerTest) TearDownSuite() {
}

func (suite *TradingServerTest) SetupSubTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))

	suite.trading = supplier.NewMockTrading(suite.T())

	suite.listener = bufconn.Listen(1024 * 1024)
	suite.server = internalGrpc.NewServer()
	server := NewTradingServer(suite.pgx, suite.trading)
	pb.RegisterTradingServer(suite.server, server)

	go func() {
		suite.NoError(suite.server.Serve(suite.listener))
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(suite.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return suite.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	suite.client = pb.NewTradingClient(conn)
}

func (test *TradingServerTest) TestGetOrders() {
	test.Run("error getting orders", func() {
		test.trading.On("GetOrders", mock.Anything, mock.Anything).Return(nil, errors.New("error getting orders"))

		req := &pb.GetOrdersRequest{}
		rsp, err := test.client.GetOrders(context.Background(), req)
		test.Require().Error(err)
		test.Require().Nil(rsp)
	})
	test.Run("normal", func() {
		orders := []*pb.Order{
			{
				Symbol: "DEMO",
				Amount: 10,
			},
		}
		test.trading.On("GetOrders", mock.Anything, mock.Anything).Return(orders, nil)

		req := &pb.GetOrdersRequest{}
		rsp, err := test.client.GetOrders(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
		test.Len(rsp.Orders, 1)
	})
}

func (test *TradingServerTest) TestPlaceOrder() {
	test.Run("error placing order", func() {
		test.trading.On("PlaceOrder", mock.Anything, mock.Anything).Return(nil, errors.New("error placing order"))

		req := &pb.PlaceOrderRequest{}
		rsp, err := test.client.PlaceOrder(context.Background(), req)
		test.Require().Error(err)
		test.Require().Nil(rsp)
	})
	test.Run("normal", func() {
		test.trading.On("PlaceOrder", mock.Anything, mock.Anything).Return(&pb.Order{}, nil)

		req := &pb.PlaceOrderRequest{}
		rsp, err := test.client.PlaceOrder(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
	})
}

func (test *TradingServerTest) TestGetPortfolio() {
	test.Run("error getting portfolio", func() {
		test.trading.On("GetPortfolio", mock.Anything, mock.Anything).Return(nil, errors.New("error getting portfolio"))

		req := &pb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().Error(err)
		test.Require().Nil(rsp)
	})
	test.Run("normal", func() {
		portfolio := &pb.Portfolio{}
		test.trading.On("GetPortfolio", mock.Anything, mock.Anything).Return(portfolio, nil)

		req := &pb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
	})
}
