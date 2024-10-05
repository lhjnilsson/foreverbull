package backtest

import (
	"context"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	backtest_pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	listener   *bufconn.Listener
	baseServer *grpc.Server
	activity   <-chan bool

	mockEngine *engine.MockEngine
	client     backtest_pb.SessionServicerClient

	backtest *backtest_pb.Backtest
	session  *backtest_pb.Session
}

func TestSessionManual(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (test *SessionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
}

func (s *SessionTest) TearDownSuite() {
}

func (s *SessionTest) SetupTest() {
	err := repository.Recreate(context.Background(), s.conn)
	s.Require().NoError(err)

	backtests := repository.Backtest{Conn: s.conn}
	s.backtest, err = backtests.Create(context.Background(), "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	sessions := repository.Session{Conn: s.conn}
	s.session, err = sessions.Create(context.TODO(), "backtest")
	s.Require().NoError(err)

	s.listener = bufconn.Listen(1024 * 1024)

	s.mockEngine = new(engine.MockEngine)
	s.baseServer, s.activity, err = NewGRPCSessionServer(s.session, s.conn, s.mockEngine)
	s.Require().NoError(err)
	go func() {
		if err := s.baseServer.Serve(s.listener); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	conn, err := grpc.DialContext(context.Background(), "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return s.listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	s.client = backtest_pb.NewSessionServicerClient(conn)

}

func (s *SessionTest) TearDownTest() {
	err := s.listener.Close()
	if err != nil {
		s.Assert().Fail("error closing listener: %v", err)
	}
	s.baseServer.Stop()
}

func (s *SessionTest) TestCreateExecution() {
	rsp, err := s.client.CreateExecution(context.Background(), &backtest_pb.CreateExecutionRequest{
		Backtest: &backtest_pb.Backtest{
			StartDate: &common_pb.Date{Year: 2024, Month: 01, Day: 01},
			EndDate:   &common_pb.Date{Year: 2024, Month: 01, Day: 01},
			Symbols:   []string{"AAPL"},
		},
		Algorithm: &service_pb.Algorithm{},
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case <-s.activity:
	case <-time.After(5 * time.Second):
		s.Require().Fail("timeout waiting for activity")
	}
}

func (s *SessionTest) TestRunExecution() {
	rsp, err := s.client.CreateExecution(context.Background(), &backtest_pb.CreateExecutionRequest{
		Backtest: &backtest_pb.Backtest{
			StartDate: &common_pb.Date{Year: 2024, Month: 01, Day: 01},
			EndDate:   &common_pb.Date{Year: 2024, Month: 01, Day: 01},
			Symbols:   []string{"AAPL"},
		},
		Algorithm: &service_pb.Algorithm{},
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case <-s.activity:
	case <-time.After(5 * time.Second):
		s.Require().Fail("timeout waiting for activity")
	}

	executions := repository.Execution{Conn: s.conn}
	execution, err := executions.Create(context.Background(), s.session.Id,
		s.backtest.StartDate, s.backtest.EndDate, []string{"AAPL"}, nil)
	s.Require().NoError(err)

	ch := make(chan *finance_pb.Portfolio, 5)
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	close(ch)
	s.mockEngine.On("RunBacktest", mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)

	stream, err := s.client.RunExecution(context.Background(), &backtest_pb.RunExecutionRequest{
		ExecutionId: execution.Id,
	})
	s.Require().NoError(err)
	s.Require().NotNil(stream)
	entries := 0
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		s.Require().NoError(err)
		entries++
	}
	s.Require().Equal(5, entries)
}

func (s *SessionTest) TestGetExecution() {
	executions := repository.Execution{Conn: s.conn}
	execution, err := executions.Create(context.TODO(), s.session.Id,
		s.backtest.StartDate, s.backtest.EndDate,
		[]string{"AAPL"}, nil)
	s.Require().NoError(err)

	rsp, err := s.client.GetExecution(context.Background(), &backtest_pb.GetExecutionRequest{
		ExecutionId: execution.Id,
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	s.Require().Equal(execution.Id, rsp.Execution.Id)
	select {
	case _, running := <-s.activity:
		s.Require().True(running)
	default:
		s.Require().Fail("activity channel should be closed")
	}
}

func (s *SessionTest) TestStopServer() {
	rsp, err := s.client.StopServer(context.Background(), &backtest_pb.StopServerRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case _, done := <-s.activity:
		s.Require().False(done)
	default:
		s.Require().Fail("activity channel should be closed")
	}
}
