package api_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	streamPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
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
	stream  *stream.MockStream

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.TradingServicerClient
}

func TestTradingServer(t *testing.T) {
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
	suite.server, err = internalGrpc.NewServer()
	suite.Require().NoError(err)
	suite.stream = new(stream.MockStream)
	server := api.NewTradingServer(suite.pgx, suite.stream, suite.trading)
	servicePb.RegisterTradingServicerServer(suite.server, server)

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

	suite.client = servicePb.NewTradingServicerClient(conn)
}

func (test *TradingServerTest) TestGetOrders() {
	test.Run("error getting orders", func() {
		test.trading.On("GetOrders", mock.Anything, mock.Anything).Return(nil, errors.New("error getting orders"))

		req := &servicePb.GetOrdersRequest{}
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

		req := &servicePb.GetOrdersRequest{}
		rsp, err := test.client.GetOrders(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
		test.Len(rsp.Orders, 1)
	})
}

func (test *TradingServerTest) TestGetPortfolio() {
	test.Run("error getting portfolio", func() {
		test.trading.On("GetPortfolio", mock.Anything, mock.Anything).Return(nil, errors.New("error getting portfolio"))

		req := &servicePb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().Error(err)
		test.Require().Nil(rsp)
	})
	test.Run("normal", func() {
		portfolio := &pb.Portfolio{}
		test.trading.On("GetPortfolio", mock.Anything, mock.Anything).Return(portfolio, nil)

		req := &servicePb.GetPortfolioRequest{}
		rsp, err := test.client.GetPortfolio(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
	})
}

func (test *TradingServerTest) TestCreateSession() {
	test.Run("normal", func() {
		repository := repository.Trading{Conn: test.pgx}
		statusCh := make(chan *streamPb.Orchestration_Step)
		go func() {
			statusCh <- &streamPb.Orchestration_Step{
				Num:   int32(servicePb.CreateSessionResponse_CREATING),
				State: commonPb.State_STATE_COMPLETED,
			}
			sessions, err := repository.List(context.Background())
			test.NoError(err)
			test.Len(sessions, 1)
			test.NoError(repository.UpdatePort(context.Background(), sessions[0].Id, 1337))
			statusCh <- &streamPb.Orchestration_Step{
				Num:   int32(servicePb.CreateSessionResponse_STARTING),
				State: commonPb.State_STATE_COMPLETED,
			}
			close(statusCh)
		}()
		test.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return((<-chan *streamPb.Orchestration_Step)(statusCh))

		req := &servicePb.CreateSessionRequest{}
		rsp, err := test.client.CreateSession(context.Background(), req)
		test.Require().NoError(err)
		test.Require().NotNil(rsp)
		var session *pb.Session
		for {
			msg, err := rsp.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			test.Require().NoError(err)
			test.Require().NotNil(msg)
			session = msg.GetSession()
		}
		test.Require().NotNil(session)
		test.Require().NotNil(session.Port)
		test.Equal(int64(1337), *session.Port)
	})
}
