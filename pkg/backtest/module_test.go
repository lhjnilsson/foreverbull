package backtest

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
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
			func() (*nats.Conn, nats.JetStreamContext, error) {
				return stream.New()
			},
			func() *pgxpool.Pool {
				return pool
			},
			func() (storage.Storage, error) {
				return storage.NewMinioStorage(context.Background())
			},
			func() *gin.Engine {
				return gin.Default()
			},
			func() *grpc.Server {
				return grpc.NewServer()
			},
			func() (container.Engine, error) {
				return container.NewEngine()
			},
		),
		fx.Invoke(
			h.NewLifeCycleRouter,
			func(lc fx.Lifecycle, g *grpc.Server) error {
				lc.Append(fx.Hook{
					OnStart: func(context.Context) error {
						listener, err := net.Listen("tcp", ":50055")
						if err != nil {
							return fmt.Errorf("failed to listen: %w", err)
						}
						go func() {
							if err := g.Serve(listener); err != nil {
								panic(err)
							}
						}()
						return nil
					},
					OnStop: func(context.Context) error {
						g.GracefulStop()
						return nil
					},
				})
				return nil
			},
		),
		stream.OrchestrationLifecycle,
		service.Module,
		finance.Module,
		Module,
	)
}

func (test *BacktestModuleTest) SetupTest() {
	test.Require().NoError(test.app.Start(context.Background()), "failed to start app")
	conn, err := grpc.NewClient("localhost:50055", grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.NoError(err, "failed to create grpc client")
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
	test.NoError(err, "failed to list backtests")
	_, err = test.ingestionClient.GetCurrentIngestion(context.TODO(), &pb.GetCurrentIngestionRequest{})
	test.NoError(err, "failed to get current ingestion")
	// Create Ingestion

	ingestion := pb.Ingestion{
		StartDate: timestamppb.New(time.Now().Add(-time.Hour * 24 * 200)),
		EndDate:   timestamppb.New(time.Now().Add(-time.Hour * 24 * 100)),
		Symbols:   []string{"AAPL", "MSFT"},
	}
	rsp, err := test.ingestionClient.CreateIngestion(context.TODO(), &pb.CreateIngestionRequest{
		Ingestion: &ingestion,
	})
	test.NoError(err, "failed to create ingestion")
	test.NotNil(rsp, "response is nil")

	time.Sleep(time.Second * 30)
	for i := 0; i < 30; i++ {
		rsp, err := test.ingestionClient.GetCurrentIngestion(context.TODO(), &pb.GetCurrentIngestionRequest{})
		test.NoError(err, "failed to get current ingestion")
		if rsp.Status != pb.IngestionStatus_READY {
			time.Sleep(time.Second / 2)
			continue
		}
		test.NotNil(rsp, "response is nil")
		test.Equal(rsp.Status, pb.IngestionStatus_READY, "status is not ready")
		test.Greater(rsp.Size, int64(0), "size is 0")
	}
	// Create Backtest
	rsp2, err := test.backtestClient.CreateBacktest(context.TODO(), &pb.CreateBacktestRequest{
		Backtest: &pb.Backtest{
			Name:      "Test Backtest",
			StartDate: timestamppb.New(time.Now().Add(-time.Hour * 24 * 200)),
			EndDate:   timestamppb.New(time.Now().Add(-time.Hour * 24 * 100)),
			Symbols:   []string{"AAPL", "MSFT"},
		},
	})
	test.NoError(err, "failed to create backtest")
	test.NotNil(rsp2, "response is nil")
	// Run Backtest
	rsp3, err := test.backtestClient.CreateSession(context.TODO(), &pb.CreateSessionRequest{BacktestName: rsp2.Backtest.Name})
	test.NoError(err, "failed to create session")
	test.NotNil(rsp3, "response is nil")
	var port int64
	for i := 0; i < 30; i++ {
		rsp, err := test.backtestClient.GetSession(context.TODO(), &pb.GetSessionRequest{SessionId: rsp3.Session.Id})
		test.NoError(err, "failed to get session")
		test.NotNil(rsp, "response is nil")
		if rsp.Session.Statuses[0].Status != pb.Session_Status_RUNNING {
			time.Sleep(time.Second / 2)
			continue
		}
		port = rsp.Session.GetPort()
		break
	}

	gCleint, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	sessionClient := pb.NewSessionServicerClient(gCleint)

	excRep, err := sessionClient.CreateExecution(context.TODO(), &pb.CreateExecutionRequest{
		Backtest:  rsp2.Backtest,
		Algorithm: &service_pb.Algorithm{},
	})
	test.Require().NoError(err, "failed to create execution")

	workerSocket, err := rep.NewSocket()
	test.NoError(err)
	err = workerSocket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", excRep.Configuration.GetBrokerPort()))
	test.Require().NoError(err, fmt.Sprintf("Failed to connect to broker port %d", excRep.Configuration.GetBrokerPort()))
	worker := func(req *service_pb.WorkerRequest) *service_pb.WorkerResponse {
		return &service_pb.WorkerResponse{}
	}
	go test_helper.WorkerSimulator(test.T(), workerSocket, worker)

	stream, err := sessionClient.RunExecution(context.TODO(), &pb.RunExecutionRequest{
		ExecutionId: excRep.Execution.Id,
	})
	test.NoError(err, "failed to run execution")
	test.NotNil(stream, "response is nil")
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		test.NoError(err, "failed to receive message")
		test.NotNil(msg, "message is nil")
	}

	rsp5, err := sessionClient.StopServer(context.TODO(), &pb.StopServerRequest{})
	test.NoError(err, "failed to stop server")
	test.NotNil(rsp5, "response is nil")
}
