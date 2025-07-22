package api_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	listener   *bufconn.Listener
	baseServer *grpc.Server
	activity   <-chan bool

	mockEngine        *engine.MockEngine
	mockEngineSession *engine.MockEngineSession
	client            servicePb.SessionServicerClient

	backtest *backtest.Backtest
	session  *backtest.Session
}

func TestSessionManual(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (s *SessionTest) SetupSuite() {
	test_helper.SetupEnvironment(s.T(), &test_helper.Containers{
		Postgres: true,
	})

	var err error
	s.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	s.Require().NoError(err)
}

func (s *SessionTest) TearDownSuite() {
}

func (s *SessionTest) SetupTest() {
	err := repository.Recreate(context.Background(), s.conn)
	s.Require().NoError(err)

	backtests := repository.Backtest{Conn: s.conn}
	backtest := backtest.Backtest{
		Name: "backtest",
	}
	s.backtest, err = backtests.Create(context.Background(), &backtest)
	s.Require().NoError(err)
	sessions := repository.Session{Conn: s.conn}
	s.session, err = sessions.Create(context.TODO(), "backtest")
	s.Require().NoError(err)

	s.listener = bufconn.Listen(1024 * 1024)

	s.mockEngine = new(engine.MockEngine)
	s.mockEngineSession = new(engine.MockEngineSession)
	s.mockEngine.On("NewSession", mock.Anything, mock.Anything).Return(s.mockEngineSession, nil)

	s.baseServer, s.activity, err = api.NewGRPCSessionServer(s.session, s.conn, s.mockEngine)
	s.Require().NoError(err)

	go func() {
		if err := s.baseServer.Serve(s.listener); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(s.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return s.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	s.client = servicePb.NewSessionServicerClient(conn)
}

func (s *SessionTest) TearDownTest() {
	err := s.listener.Close()
	if err != nil {
		s.Failf("error closing listener: %s", err.Error())
	}

	s.baseServer.Stop()
}

func (s *SessionTest) TestCreateExecution() {
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Backtest:  &backtest.Backtest{},
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
	rsp, err := s.client.CreateExecution(context.Background(), &servicePb.CreateExecutionRequest{
		Backtest:  &backtest.Backtest{},
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
	e := backtest.Execution{
		Session: s.session.Id,
	}
	execution, err := executions.Create(context.Background(), &e)
	s.Require().NoError(err)

	ch := make(chan *servicePb.RunExecutionResponse, 5)
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	ch <- &servicePb.RunExecutionResponse{}
	close(ch)
	s.mockEngineSession.On("RunBacktest", mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)

	stream, err := s.client.RunExecution(context.Background(), &servicePb.RunExecutionRequest{
		ExecutionId: execution.Id,
	})
	s.Require().NoError(err)
	s.Require().NotNil(stream)

	entries := 0

	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		s.Require().NoError(err)

		entries++
	}

	s.Require().Equal(5, entries)
}

func (s *SessionTest) TestStopServer() {
	rsp, err := s.client.StopServer(context.Background(), &servicePb.StopServerRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(rsp)
	select {
	case _, done := <-s.activity:
		s.Require().False(done)
	default:
		s.Require().Fail("activity channel should be closed")
	}
}
