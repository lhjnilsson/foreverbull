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
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
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
	client   servicePb.BacktestServicerClient
}

func TestBacktestServerTest(t *testing.T) {
	suite.Run(t, new(BacktestServerTest))
}

func (test *BacktestServerTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *BacktestServerTest) SetupSubTest() {
	var err error
	test.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	test.Require().NoError(repository.Recreate(context.TODO(), test.pgx))

	test.listener = bufconn.Listen(1024 * 1024)
	test.server, err = internalGrpc.NewServer()
	test.Require().NoError(err)

	test.stream = new(stream.MockStream)
	server := api.NewBacktestServer(test.pgx, test.stream)
	servicePb.RegisterBacktestServicerServer(test.server, server)

	go func() {
		test.server.Serve(test.listener) // nolint:errcheck
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(test.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return test.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	test.client = servicePb.NewBacktestServicerClient(conn)
}

func (test *BacktestServerTest) TearDownSubTest() {
	err := test.listener.Close()
	if err != nil {
		test.T().Errorf("Error closing listener: %v", err)
	}

	test.server.Stop()
}

func (test *BacktestServerTest) createBacktest(name string) *pb.Backtest {
	test.T().Helper()

	backtests := repository.Backtest{Conn: test.pgx}
	backtest, err := backtests.Create(context.TODO(), name, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{"AAPL"}, nil)
	test.Require().NoError(err)
	test.Require().NotNil(backtest)
	return backtest
}

func (test *BacktestServerTest) createSession(backtest string) *pb.Session {
	test.T().Helper()

	sessions := repository.Session{Conn: test.pgx}
	session, err := sessions.Create(context.TODO(), backtest)
	test.Require().NoError(err)
	test.Require().NotNil(session)

	return session
}

func (test *BacktestServerTest) createExecution(session string) *pb.Execution {
	test.T().Helper()

	executions := repository.Execution{Conn: test.pgx}
	execution, err := executions.Create(context.TODO(),
		session, &common_pb.Date{Year: 2024, Month: 1, Day: 1},
		&common_pb.Date{Year: 2024, Month: 1, Day: 1},
		[]string{"AAPL"}, nil)
	test.Require().NoError(err)
	test.Require().NotNil(execution)

	return execution
}

func (test *BacktestServerTest) TestListBacktests() {
	test.Run("List", func() {
		req := &servicePb.ListBacktestsRequest{}

		resp, err := test.client.ListBacktests(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
	})
}

func (test *BacktestServerTest) TestCreateBacktest() {
	test.Run("Create", func() {
		statuses := []*streamPb.Orchestration_Step{
			{Num: 2, State: common_pb.State_STATE_RUNNING},
			{Num: 2, State: common_pb.State_STATE_COMPLETED},
		}

		statusCh := make(chan *streamPb.Orchestration_Step, len(statuses))
		for _, status := range statuses {
			statusCh <- status
		}
		close(statusCh)
		test.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return((<-chan *streamPb.Orchestration_Step)(statusCh))

		req := &servicePb.CreateBacktestRequest{
			Backtest: &pb.Backtest{
				Name:      "Test Backtest",
				StartDate: &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				EndDate:   &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				Symbols:   []string{"AAPL", "MSFT"},
			},
		}

		resp, err := test.client.CreateBacktest(context.TODO(), req)
		test.Require().NoError(err)
		test.NotNil(resp)

		responses := []*servicePb.CreateBacktestResponse{}

		for {
			msg, err := resp.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			test.Require().NoError(err)
			test.Require().NotNil(msg)
			responses = append(responses, msg)
		}
		test.Require().Len(responses, 2)
	})
}

func (test *BacktestServerTest) TestGetBacktest() {
	test.Run("not found", func() {
		req := &servicePb.GetBacktestRequest{
			Name: "not_found",
		}
		resp, err := test.client.GetBacktest(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("found", func() {
		backtest := test.createBacktest("test_1")

		req := &servicePb.GetBacktestRequest{
			Name: backtest.Name,
		}

		resp, err := test.client.GetBacktest(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Equal(backtest, resp.Backtest)
	})
}

func (test *BacktestServerTest) TestCreateSession() {
	test.Run("backtest not found", func() {

		req := &servicePb.CreateSessionRequest{
			BacktestName: "backtest",
		}

		resp, err := test.client.CreateSession(context.TODO(), req)
		test.Require().NoError(err)
		test.Require().NotNil(resp)

		msg, err := resp.Recv()
		test.Require().Error(err)
		test.Require().Nil(msg)
	})

	test.Run("normal", func() {
		backtest := test.createBacktest("test_1")

		test.stream.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		req := &servicePb.CreateSessionRequest{
			BacktestName: backtest.Name,
		}

		resp, err := test.client.CreateSession(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)

		responses := []*servicePb.CreateSessionResponse{}
		for {
			msg, err := resp.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			test.Require().NoError(err)
			test.Require().NotNil(msg)
			responses = append(responses, msg)
		}
		test.Require().Len(responses, 2)
	})
}

func (test *BacktestServerTest) TestGetSession() {
	test.Run("not found", func() {
		req := &servicePb.GetSessionRequest{
			SessionId: "not_found",
		}
		resp, err := test.client.GetSession(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("found", func() {
		backtest := test.createBacktest("test_1")
		session := test.createSession(backtest.Name)

		req := &servicePb.GetSessionRequest{
			SessionId: session.Id,
		}

		resp, err := test.client.GetSession(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
	})
}

func (test *BacktestServerTest) TestListExecutions() {
	test.Run("List by session", func() {
		backtest := test.createBacktest("test_1")
		session := test.createSession(backtest.Name)
		execution := test.createExecution(session.Id)

		req := &servicePb.ListExecutionsRequest{
			SessionId: session.Id,
		}
		resp, err := test.client.ListExecutions(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Len(resp.Executions, 1)
		test.Equal(execution.Id, resp.Executions[0].Id)
	})
	test.Run("List by backtest", func() {
		backtest := test.createBacktest("test_1")
		session := test.createSession(backtest.Name)
		execution := test.createExecution(session.Id)

		req := &servicePb.ListExecutionsRequest{
			Backtest: backtest.Name,
		}
		resp, err := test.client.ListExecutions(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Len(resp.Executions, 1)
		test.Equal(execution.Id, resp.Executions[0].Id)
	})
	test.Run("List without filter", func() {
		backtest := test.createBacktest("test_1")
		session := test.createSession(backtest.Name)
		execution := test.createExecution(session.Id)

		req := &servicePb.ListExecutionsRequest{}
		resp, err := test.client.ListExecutions(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Len(resp.Executions, 1)
		test.Equal(execution.Id, resp.Executions[0].Id)
	})
}

func (test *BacktestServerTest) TestGetExecution() {
	test.Run("not found", func() {
		req := &servicePb.GetExecutionRequest{
			ExecutionId: "not_found",
		}
		resp, err := test.client.GetExecution(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("found", func() {
		backtest := test.createBacktest("test_1")
		session := test.createSession(backtest.Name)
		execution := test.createExecution(session.Id)

		req := &servicePb.GetExecutionRequest{
			ExecutionId: execution.Id,
		}
		resp, err := test.client.GetExecution(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Equal(execution, resp.Execution)
	})
}
