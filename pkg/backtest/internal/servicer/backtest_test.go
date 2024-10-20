package servicer_test

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/servicer"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type BacktestServerTest struct {
	suite.Suite

	pgx    *pgxpool.Pool
	stream *stream.MockStream

	listener *bufconn.Listener
	server   *grpc.Server
	client   pb.BacktestServicerClient
}

func TestBacktestServerTest(t *testing.T) {
	suite.Run(t, new(BacktestServerTest))
}

func (suite *BacktestServerTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (suite *BacktestServerTest) TearDownSuite() {
}

func (suite *BacktestServerTest) SetupTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))

	suite.listener = bufconn.Listen(1024 * 1024)
	suite.server = grpc.NewServer()

	suite.stream = new(stream.MockStream)
	server := servicer.NewBacktestServer(suite.pgx, suite.stream)
	pb.RegisterBacktestServicerServer(suite.server, server)

	go func() {
		suite.server.Serve(suite.listener)
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

	suite.client = pb.NewBacktestServicerClient(conn)
}

func (suite *BacktestServerTest) TearDownTest() {
	err := suite.listener.Close()
	if err != nil {
		suite.T().Errorf("Error closing listener: %v", err)
	}

	suite.server.Stop()
}

func (suite *BacktestServerTest) createBacktest(name string) *pb.Backtest {
	suite.T().Helper()

	backtests := repository.Backtest{Conn: suite.pgx}
	backtest, err := backtests.Create(context.TODO(), name, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{"AAPL"}, nil)
	suite.Require().NoError(err)
	suite.Require().NotNil(backtest)

	return backtest
}

func (suite *BacktestServerTest) createSession(backtest string) *pb.Session {
	suite.T().Helper()

	sessions := repository.Session{Conn: suite.pgx}
	session, err := sessions.Create(context.TODO(), backtest)
	suite.Require().NoError(err)
	suite.Require().NotNil(session)

	return session
}

func (suite *BacktestServerTest) TestListBacktests() {
	req := &pb.ListBacktestsRequest{}

	resp, err := suite.client.ListBacktests(context.Background(), req)
	suite.Require().NoError(err)
	suite.NotNil(resp)
}

func (suite *BacktestServerTest) TestCreateBacktest() {
	req := &pb.CreateBacktestRequest{
		Backtest: &pb.Backtest{
			Name:      "test_1",
			StartDate: &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
			EndDate:   &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
			Symbols:   []string{"AAPL"},
			Benchmark: nil,
		},
	}

	resp, err := suite.client.CreateBacktest(context.Background(), req)
	suite.Require().NoError(err)
	suite.NotNil(resp)
}

func (suite *BacktestServerTest) TestGetBacktest() {
	backtest := suite.createBacktest("test_1")

	req := &pb.GetBacktestRequest{
		Name: backtest.Name,
	}

	resp, err := suite.client.GetBacktest(context.Background(), req)
	suite.Require().NoError(err)
	suite.NotNil(resp)
	suite.Equal(backtest, resp.Backtest)
}

func (suite *BacktestServerTest) TestCreateSession() {
	backtest := suite.createBacktest("test_1")

	suite.stream.On("Publish", mock.Anything, mock.Anything).Return(nil)

	req := &pb.CreateSessionRequest{
		BacktestName: backtest.Name,
	}

	resp, err := suite.client.CreateSession(context.Background(), req)
	suite.Require().NoError(err)
	suite.NotNil(resp)
}

func (suite *BacktestServerTest) TestGetSession() {
	backtest := suite.createBacktest("test_1")
	session := suite.createSession(backtest.Name)

	req := &pb.GetSessionRequest{
		SessionId: session.Id,
	}

	resp, err := suite.client.GetSession(context.Background(), req)
	suite.Require().NoError(err)
	suite.NotNil(resp)
}
