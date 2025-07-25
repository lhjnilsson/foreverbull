//go:build integration

package backtest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	service_servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/service/service"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BacktestModuleTest struct {
	suite.Suite
	app *fx.App

	backtestClient servicePb.BacktestServicerClient
}

func TestModuleBacktest(t *testing.T) {
	suite.Run(t, new(BacktestModuleTest))
}

func (test *BacktestModuleTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
		Minio:    true,
		Loki:     true,
		Zipline:  true,
	})

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.Require().NoError(err)

	test.app = fx.New(
		fx.Provide(
			func() *pgxpool.Pool {
				return pool
			},
			func() (storage.Storage, error) {
				return storage.NewMinioStorage(context.Background())
			},
			container.NewEngine,
		),
		fx.Invoke(),
		internalGrpc.Module,
		stream.Module,
		service.Module,
		finance.Module,
		backtest.Module,
	)
}

func (test *BacktestModuleTest) SetupTest() {
	test.Require().NoError(test.app.Start(context.Background()), "failed to start app")

	conn, err := grpc.NewClient(environment.GetServerGRPCURL(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.Require().NoError(err, "failed to create grpc client")
	test.backtestClient = servicePb.NewBacktestServicerClient(conn)
}

func (test *BacktestModuleTest) TearDownTest() {
	test.NoError(test.app.Stop(context.Background()), "failed to stop app")
}

func (test *BacktestModuleTest) TearDownSuite() {
	test_helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()), "failed to stop app")
}

func (test *BacktestModuleTest) TestBacktestModule() {
	// Create Backtest
	createBacktestStream, err := test.backtestClient.CreateBacktest(context.TODO(), &servicePb.CreateBacktestRequest{
		Name:      "Test Backtest",
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		BaseEtf:   "iShares S&P 100",
	})
	test.Require().NoError(err, "failed to create backtest: %v", err)

	var createBacktest *servicePb.CreateBacktestResponse
	for {
		rsp, err := createBacktestStream.Recv()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		test.Require().NoError(err)
		createBacktest = rsp
	}

	test.Require().NotNil(createBacktest, "response is nil")
	test.Require().Equal(pb.State_STATE_COMPLETED, createBacktest.INGESTING.State)

	getResponse, err := test.backtestClient.GetBacktest(context.TODO(), &servicePb.GetBacktestRequest{Name: "Test Backtest"})
	test.Require().NoError(err, "failed to get backtest")
	test.NotNil(getResponse, "response is nil")

	// Run Backtest
	rsp3, err := test.backtestClient.CreateSession(context.TODO(),
		&servicePb.CreateSessionRequest{BacktestName: getResponse.GetName()})
	test.Require().NoError(err, "failed to create session")
	test.NotNil(rsp3, "response is nil")

	var latestSessionRsp *servicePb.CreateSessionResponse

	for {
		rsp, err := rsp3.Recv()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		test.Require().NoError(err, "failed to receive response")
		latestSessionRsp = rsp
	}

	test.Require().NotNil(latestSessionRsp, "response is nil")
	test.Require().NotNil(latestSessionRsp.Session, "session is nil")

	test.Require().NotNil(latestSessionRsp.Session.Port)
	port := *latestSessionRsp.Session.Port

	var gClient *grpc.ClientConn
	gClient, err = grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	sessionClient := servicePb.NewSessionServicerClient(gClient)

	test.Require().NoError(err)

	cb := func(_ *service_servicePb.WorkerRequest) *service_servicePb.WorkerResponse {
		test.T().Log("WorkerSimulator callback")
		return &service_servicePb.WorkerResponse{}
	}
	functions := []*test_helper.WorkerFunction{
		{
			CB:       cb,
			Name:     "test",
			Parallel: true,
		},
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), functions...)

	excRep, err := sessionClient.CreateExecution(context.TODO(), &servicePb.CreateExecutionRequest{
		Id:        "execution_123",
		StartDate: getResponse.Backtest.Periods[0].StartDate,
		EndDate:   getResponse.Backtest.Periods[(len(getResponse.Backtest.Periods) - 1)].EndDate,
		Algorithm: algo,
	})
	test.Require().NoError(err, "failed to create execution")

	workerSocket, err := rep.NewSocket()
	test.Require().NoError(err)
	err = workerSocket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", excRep.Configuration.GetBrokerPort()))
	test.Require().NoError(err, "Failed to connect to broker port ", excRep.Configuration.GetBrokerPort())

	go runner(workerSocket)
	defer workerSocket.Close()

	stream, err := sessionClient.RunExecution(context.TODO(), &servicePb.RunExecutionRequest{
		ExecutionId: excRep.Execution.Id,
	})
	test.Require().NoError(err, "failed to run execution")
	test.NotNil(stream, "response is nil")

	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		test.Require().NoError(err, "failed to receive message")
		test.Require().NotNil(msg, "message is nil")
	}

	rspResult, err := sessionClient.StoreResult(context.TODO(), &servicePb.StoreExecutionResultRequest{
		ExecutionId: excRep.Execution.Id,
	})
	test.Require().NoError(err, "failed to store result")
	test.NotNil(rspResult, "response is nil")

	rsp5, err := sessionClient.StopServer(context.TODO(), &servicePb.StopServerRequest{})
	test.Require().NoError(err, "failed to stop server")
	test.NotNil(rsp5, "response is nil")
}
