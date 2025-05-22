package backtest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
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
	backtestImage := os.Getenv("BACKTEST_IMAGE")
	if backtestImage == "" {
		t.Skip("backtest image not set")
	}

	suite.Run(t, new(BacktestModuleTest))
}

func (test *BacktestModuleTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
		Minio:    true,
		Loki:     true,
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
			func() *grpc.Server {
				return grpc.NewServer()
			},
			container.NewEngine,
		),
		fx.Invoke(
			func(lc fx.Lifecycle, gServer *grpc.Server) error {
				lc.Append(fx.Hook{
					OnStart: func(context.Context) error {
						listener, err := net.Listen("tcp", fmt.Sprintf(":%s", environment.GetGRPCPort()))
						if err != nil {
							return fmt.Errorf("failed to listen: %w", err)
						}
						go func() {
							if err := gServer.Serve(listener); err != nil {
								panic(err)
							}
						}()
						return nil
					},
					OnStop: func(context.Context) error {
						gServer.GracefulStop()
						return nil
					},
				})

				return nil
			},
		),
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
	_, exists := os.LookupEnv("BACKTEST_IMAGE")
	if !exists {
		test.T().Skip("IMAGES not set")
	}

	// Create Backtest
	createResponse, err := test.backtestClient.CreateBacktest(context.TODO(), &servicePb.CreateBacktestRequest{
		Backtest: &pb.Backtest{
			Name:      "Test Backtest",
			StartDate: common_pb.GoTimeToDate(time.Now().Add(-time.Hour * 24 * 200)),
			EndDate:   common_pb.GoTimeToDate(time.Now().Add(-time.Hour * 24)),
			Symbols:   []string{"AAPL", "MSFT"},
		},
	})
	test.Require().NoError(err, "failed to create backtest: %v", err)

	var latestStatus *servicePb.CreateBacktestResponse_Status
	for {
		rsp, err := createResponse.Recv()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		test.Require().NoError(err)
		latestStatus = rsp.Status
	}

	test.Require().NotNil(latestStatus)
	test.Require().Equal(pb.BacktestStatus_INGESTING, latestStatus.Status)
	test.Require().Equal(common_pb.State_STATE_COMPLETED, latestStatus.State)

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
	test.Require().Equal(pb.SessionStatus_RUNNING, latestSessionRsp.Status.Status)

	test.Require().NotNil(latestSessionRsp.Session.Port)
	port := *latestSessionRsp.Session.Port

	var gClient *grpc.ClientConn
	gClient, err = grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	sessionClient := servicePb.NewSessionServicerClient(gClient)

	test.Require().NoError(err)

	cb := func(_ *service_servicePb.WorkerRequest) *service_servicePb.WorkerResponse {
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
		Backtest:  getResponse.GetBacktest(),
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
