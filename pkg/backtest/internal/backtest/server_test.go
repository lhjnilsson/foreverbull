package backtest

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	backtest_pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	serviceEntity "github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type SessionManualTest struct {
	suite.Suite

	conn *pgxpool.Pool

	listener   *bufconn.Listener
	baseServer *grpc.Server
	activity   <-chan bool

	mockEngine     *engine.MockEngine
	mockWorkerPool *worker.MockPool
	client         backtest_pb.SessionServicerClient

	backtest *backtest_pb.Backtest
	session  *backtest_pb.Session
}

func TestSessionManual(t *testing.T) {
	suite.Run(t, new(SessionManualTest))
}

func (test *SessionManualTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	test.mockWorkerPool = new(worker.MockPool)
}

func (s *SessionManualTest) TearDownSuite() {
}

func (s *SessionManualTest) SetupTest() {
	err := repository.Recreate(context.Background(), s.conn)
	s.Require().NoError(err)

	backtests := repository.Backtest{Conn: s.conn}
	s.backtest, err = backtests.Create(context.Background(), "backtest", time.Now(), time.Now(), []string{}, nil)
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

func (s *SessionManualTest) TearDownTest() {
	err := s.listener.Close()
	if err != nil {
		s.Assert().Fail("error closing listener: %v", err)
	}
	s.baseServer.Stop()
}

func (s *SessionManualTest) TestCreateExecution() {
	s.mockWorkerPool.On("GetAlgorithm").Return(&serviceEntity.Algorithm{})
	s.mockWorkerPool.On("GetPort").Return(0)
	s.mockWorkerPool.On("GetNamespacePort").Return(1)

	rsp, err := s.client.CreateExecution(context.Background(), &backtest_pb.CreateExecutionRequest{
		Backtest: &backtest_pb.Backtest{
			StartDate: pb.TimeToProtoTimestamp(time.Now()),
			EndDate:   pb.TimeToProtoTimestamp(time.Now()),
			Symbols:   []string{"AAPL"},
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case <-s.activity:
	case <-time.After(5 * time.Second):
		s.Require().Fail("timeout waiting for activity")
	}
}

func (s *SessionManualTest) TestRunExecution() {
	ch := make(chan *finance_pb.Portfolio, 5)
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	ch <- &finance_pb.Portfolio{}
	close(ch)
	s.mockEngine.On("RunBacktest", mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)

	stream, err := s.client.RunExecution(context.Background(), &backtest_pb.RunExecutionRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(stream)
	entries := 0
	for {
		_, err := stream.Recv()
		if err != nil {
			break
		}
		entries++
	}
	s.Require().Equal(5, entries)
}

func (s *SessionManualTest) TestGetExecution() {
	executions := repository.Execution{Conn: s.conn}
	execution, err := executions.Create(context.TODO(), s.session.Id,
		s.backtest.StartDate.AsTime(), s.backtest.EndDate.AsTime(),
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

func (s *SessionManualTest) TestStopServer() {
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
