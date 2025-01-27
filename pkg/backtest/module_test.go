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
	service_pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
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

	backtestClient  pb.BacktestServicerClient
	ingestionClient pb.IngestionServicerClient
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
			stream.New,
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
		stream.OrchestrationLifecycle,
		service.Module,
		finance.Module,
		backtest.Module,
	)
}

func (test *BacktestModuleTest) SetupTest() {
	test.Require().NoError(test.app.Start(context.Background()), "failed to start app")

	conn, err := grpc.NewClient(environment.GetServerGRPCURL(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.Require().NoError(err, "failed to create grpc client")
	test.backtestClient = pb.NewBacktestServicerClient(conn)
	test.ingestionClient = pb.NewIngestionServicerClient(conn)
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

	_, err := test.backtestClient.ListBacktests(context.Background(), &pb.ListBacktestsRequest{})
	test.Require().NoError(err, "failed to list backtests")
	_, err = test.ingestionClient.GetCurrentIngestion(context.TODO(), &pb.GetCurrentIngestionRequest{})
	test.Require().NoError(err, "failed to get current ingestion")

	// Create Backtest
	rsp2, err := test.backtestClient.CreateBacktest(context.TODO(), &pb.CreateBacktestRequest{
		Backtest: &pb.Backtest{
			Name:      "Test Backtest",
			StartDate: common_pb.GoTimeToDate(time.Now().Add(-time.Hour * 24 * 200)),
			EndDate:   common_pb.GoTimeToDate(time.Now().Add(-time.Hour * 24)),
			Symbols:   []string{"AAPL", "MSFT"},
		},
	})
	test.Require().NoError(err, "failed to create backtest")
	test.NotNil(rsp2, "response is nil")

	// Create Ingestion
	is, err := test.ingestionClient.UpdateIngestion(context.TODO(), &pb.UpdateIngestionRequest{})
	test.Require().NoError(err, "failed to create ingestion")
	test.NotNil(is, "response is nil")

	for {
		rsp, err := is.Recv()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		test.Require().NoError(err)
		test.Require().NotEqual(pb.IngestionStatus_ERROR, rsp.Status)
	}

	// Run Backtest
	rsp3, err := test.backtestClient.CreateSession(context.TODO(),
		&pb.CreateSessionRequest{BacktestName: rsp2.Backtest.Name})
	test.Require().NoError(err, "failed to create session")
	test.NotNil(rsp3, "response is nil")

	var port int64

	for range 30 {
		rsp, err := test.backtestClient.GetSession(context.TODO(), &pb.GetSessionRequest{SessionId: rsp3.Session.Id})
		test.Require().NoError(err, "failed to get session")
		test.NotNil(rsp, "response is nil")

		if rsp.Session.Statuses[0].Status != pb.Session_Status_RUNNING {
			time.Sleep(time.Second / 2)
			continue
		}

		port = rsp.Session.GetPort()

		break
	}

	var gClient *grpc.ClientConn
	gClient, err = grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	sessionClient := pb.NewSessionServicerClient(gClient)

	test.Require().NoError(err)

	cb := func(_ *service_pb.WorkerRequest) *service_pb.WorkerResponse {
		return &service_pb.WorkerResponse{}
	}
	functions := []*test_helper.WorkerFunction{
		{
			CB:       cb,
			Name:     "test",
			Parallel: true,
		},
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), functions...)

	excRep, err := sessionClient.CreateExecution(context.TODO(), &pb.CreateExecutionRequest{
		Backtest:  rsp2.Backtest,
		Algorithm: algo,
	})
	test.Require().NoError(err, "failed to create execution")

	workerSocket, err := rep.NewSocket()
	test.Require().NoError(err)
	err = workerSocket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", excRep.Configuration.GetBrokerPort()))
	test.Require().NoError(err, "Failed to connect to broker port ", excRep.Configuration.GetBrokerPort())

	go runner(workerSocket)
	defer workerSocket.Close()

	stream, err := sessionClient.RunExecution(context.TODO(), &pb.RunExecutionRequest{
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

	rspResult, err := sessionClient.StoreResult(context.TODO(), &pb.StoreExecutionResultRequest{
		ExecutionId: excRep.Execution.Id,
	})
	test.Require().NoError(err, "failed to store result")
	test.NotNil(rspResult, "response is nil")

	rsp5, err := sessionClient.StopServer(context.TODO(), &pb.StopServerRequest{})
	test.Require().NoError(err, "failed to stop server")
	test.NotNil(rsp5, "response is nil")
}
