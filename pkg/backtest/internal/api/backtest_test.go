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
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
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
	pb.RegisterBacktestServicerServer(test.server, server)

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

	test.client = pb.NewBacktestServicerClient(conn)
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
		req := &pb.ListBacktestsRequest{}

		resp, err := test.client.ListBacktests(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
	})
}

func (test *BacktestServerTest) TestCreateBacktest() {
	updateBacktestStatus := func(name string, status pb.BacktestStatus) {
		backtests := repository.Backtest{Conn: test.pgx}
		err := backtests.UpdateStatus(context.Background(), name, status, nil)
		test.Require().NoError(err)
	}
	runTest := func(req *pb.CreateBacktestRequest, statuses []pb.BacktestStatus) error {
		resp, err := test.client.CreateBacktest(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		defer func() {
			test.Require().NoError(resp.CloseSend())
		}()
		msg, err := resp.Recv()
		if err != nil {
			return err
		}
		test.Require().NotNil(msg)
		test.Require().Equal(pb.BacktestStatus_CREATED, msg.GetBacktest().GetStatuses()[0].GetStatus())

		for _, status := range statuses {
			updateBacktestStatus(req.GetBacktest().GetName(), status)
			msg, err := resp.Recv()
			test.Require().NoError(err)
			test.Require().NotNil(msg)
			test.Require().Equal(status, msg.GetBacktest().GetStatuses()[0].GetStatus())
		}
		_, err = resp.Recv()
		test.Require().Error(err)
		test.Require().Equal(io.EOF, err)
		return nil
	}

	test.Run("Ready", func() {
		req := &pb.CreateBacktestRequest{
			Backtest: &pb.Backtest{
				Name:      "test_1",
				StartDate: &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				EndDate:   &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				Symbols:   []string{"AAPL"},
				Benchmark: nil,
			},
		}

		test.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return(nil)
		test.Require().NoError(runTest(req, []pb.BacktestStatus{pb.BacktestStatus_READY}))
		test.stream.AssertCalled(test.T(), "RunOrchestration", mock.Anything, mock.Anything)
	})
	test.Run("Error", func() {
		req := &pb.CreateBacktestRequest{
			Backtest: &pb.Backtest{
				Name:      "test_3",
				StartDate: &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				EndDate:   &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				Symbols:   []string{"AAPL"},
				Benchmark: nil,
			},
		}
		test.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return(nil)
		test.Require().NoError(runTest(req, []pb.BacktestStatus{pb.BacktestStatus_ERROR}))
		test.stream.AssertCalled(test.T(), "RunOrchestration", mock.Anything, mock.Anything)
	})
	test.Run("Fail to create orchestration", func() {
		req := &pb.CreateBacktestRequest{
			Backtest: &pb.Backtest{
				Name:      "test_2",
				StartDate: &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				EndDate:   &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				Symbols:   []string{"AAPL"},
				Benchmark: nil,
			},
		}
		test.stream.On("RunOrchestration", mock.Anything, mock.Anything).Return(errors.New("fail_to_create_orchestration"))
		test.ErrorContains(runTest(req, []pb.BacktestStatus{}), "fail_to_create_orchestration")
	})
}

func (test *BacktestServerTest) TestGetBacktest() {
	test.Run("not found", func() {
		req := &pb.GetBacktestRequest{
			Name: "not_found",
		}
		resp, err := test.client.GetBacktest(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("found", func() {
		backtest := test.createBacktest("test_1")

		req := &pb.GetBacktestRequest{
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
		req := &pb.CreateSessionRequest{
			BacktestName: "backtest",
		}

		resp, err := test.client.CreateSession(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("fail to publish", func() {
		backtest := test.createBacktest("test_1")

		test.stream.On("Publish", mock.Anything, mock.Anything).Return(errors.New("fail_to_publish"))

		req := &pb.CreateSessionRequest{
			BacktestName: backtest.Name,
		}

		resp, err := test.client.CreateSession(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("normal", func() {
		backtest := test.createBacktest("test_1")

		test.stream.On("Publish", mock.Anything, mock.Anything).Return(nil)

		req := &pb.CreateSessionRequest{
			BacktestName: backtest.Name,
		}

		resp, err := test.client.CreateSession(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
	})
}

func (test *BacktestServerTest) TestGetSession() {
	test.Run("not found", func() {
		req := &pb.GetSessionRequest{
			SessionId: "not_found",
		}
		resp, err := test.client.GetSession(context.Background(), req)
		test.Require().Error(err)
		test.Nil(resp)
	})
	test.Run("found", func() {
		backtest := test.createBacktest("test_1")
		session := test.createSession(backtest.Name)

		req := &pb.GetSessionRequest{
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

		req := &pb.ListExecutionsRequest{
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

		req := &pb.ListExecutionsRequest{
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

		req := &pb.ListExecutionsRequest{}
		resp, err := test.client.ListExecutions(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Len(resp.Executions, 1)
		test.Equal(execution.Id, resp.Executions[0].Id)
	})
}

func (test *BacktestServerTest) TestGetExecution() {
	test.Run("not found", func() {
		req := &pb.GetExecutionRequest{
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

		req := &pb.GetExecutionRequest{
			ExecutionId: execution.Id,
		}
		resp, err := test.client.GetExecution(context.Background(), req)
		test.Require().NoError(err)
		test.NotNil(resp)
		test.Equal(execution, resp.Execution)
	})
}
