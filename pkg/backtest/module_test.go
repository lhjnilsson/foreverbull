package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	backtest_pb "github.com/lhjnilsson/foreverbull/internal/pb/backtest"
	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	serviceAPI "github.com/lhjnilsson/foreverbull/pkg/service/api"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	repSocket "go.nanomsg.org/mangos/v3/protocol/rep"
	reqSocket "go.nanomsg.org/mangos/v3/protocol/req"
	"go.uber.org/fx"
	"google.golang.org/protobuf/proto"
)

type BacktestModuleTest struct {
	suite.Suite
	app *fx.App

	backtestName string
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
			func() *gin.Engine {
				return h.NewEngine()
			},
			func() (storage.BlobStorage, error) {
				return storage.NewMinioStorage()
			},
		),
		fx.Invoke(
			h.NewLifeCycleRouter,
		),
		stream.OrchestrationLifecycle,
		service.Module,
		finance.Module,
		Module,
	)
	test.Require().NoError(test.app.Start(context.Background()))
	payload := `{"symbols":["AAPL"],"calendar": "XNYS", "start":"2020-01-01T00:00:00Z","end":"2020-01-31T00:00:00Z"}`
	rsp := test_helper.Request(test.T(), http.MethodPost, "/backtest/api/ingestion", payload)
	if !test.Equal(http.StatusCreated, rsp.StatusCode) {
		rspData, _ := io.ReadAll(rsp.Body)
		test.Failf("Failed to ingest data: %s", string(rspData))
	}
	condition := func() (bool, error) {
		type BacktestResponse struct {
			Statuses []struct {
				Status string
			}
		}
		rsp := test_helper.Request(test.T(), http.MethodGet, "/backtest/api/ingestion", nil)
		if rsp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("failed to get backtest: %d", rsp.StatusCode)
		}
		data := &BacktestResponse{}
		err := json.NewDecoder(rsp.Body).Decode(data)
		if err != nil {
			return false, fmt.Errorf("failed to decode response: %s", err.Error())
		}
		if data.Statuses[0].Status == string(entity.IngestionStatusError) {
			return false, fmt.Errorf("backtest failed")
		}

		if data.Statuses[0].Status == string(entity.IngestionStatusCompleted) {
			return true, nil
		}
		return false, nil
	}
	test.Require().NoError(test_helper.WaitUntilCondition(test.T(), condition, time.Second*30))

	payload = `{"name":"test","symbols":["AAPL"],"calendar": "XNYS", "start":"2020-01-01T00:00:00Z","end":"2020-01-31T00:00:00Z"}`
	rsp = test_helper.Request(test.T(), http.MethodPost, "/backtest/api/backtests", payload)
	if !test.Equal(http.StatusCreated, rsp.StatusCode) {
		rspData, _ := io.ReadAll(rsp.Body)
		test.Failf("Failed to create backtest: %s", string(rspData))
	}
	test.backtestName = "test"
}

func (test *BacktestModuleTest) TearDownSuite() {
	test_helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()), "failed to stop app")
}

func (test *BacktestModuleTest) TestRunBacktestAutomatic() {
	images, exists := os.LookupEnv("IMAGES")
	if !exists {
		test.T().Skip("IMAGES not set")
	}

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	for _, image := range strings.Split(images, ",") {
		test.Run(image, func() {
			sAPI, err := serviceAPI.NewClient()
			test.Require().NoError(err)
			_, err = sAPI.CreateService(context.Background(), &serviceAPI.CreateServiceRequest{
				Image: image,
			})
			test.Require().NoError(err)

			_, err = pool.Exec(context.Background(), "UPDATE backtest SET service=$1 WHERE name=$2", image, test.backtestName)
			test.Require().NoError(err)

			type SessionResponse struct {
				ID       string
				Statuses []struct {
					Status string
				}
			}

			payload := `{"backtest": "test", "executions": [{}]}`
			rsp := test_helper.Request(test.T(), http.MethodPost, "/backtest/api/sessions", payload)
			if !test.Equal(http.StatusCreated, rsp.StatusCode) {
				rspData, _ := io.ReadAll(rsp.Body)
				test.Failf("Failed to create session: %s", string(rspData))
			}
			data := &SessionResponse{}
			err = json.NewDecoder(rsp.Body).Decode(data)
			if err != nil {
				test.Failf("Failed to decode response: %s", err.Error())
			}
			condition := func() (bool, error) {
				rsp := test_helper.Request(test.T(), http.MethodGet, "/backtest/api/sessions/"+data.ID, nil)
				if rsp.StatusCode != http.StatusOK {
					return false, fmt.Errorf("failed to get session: %d", rsp.StatusCode)
				}
				data := &SessionResponse{}
				err := json.NewDecoder(rsp.Body).Decode(data)
				if err != nil {
					return false, fmt.Errorf("failed to decode response: %s", err.Error())
				}
				if data.Statuses[0].Status == string(entity.SessionStatusCompleted) {
					return true, nil
				}
				if data.Statuses[0].Status == string(entity.SessionStatusFailed) {
					return false, fmt.Errorf("backtest failed")
				}
				return false, nil
			}
			test.NoError(test_helper.WaitUntilCondition(test.T(), condition, time.Second*30))
		})
	}
}

func (test *BacktestModuleTest) TestRunBacktestManual() {
	type SessionResponse struct {
		ID       string
		Statuses []struct {
			Status string
		}
		Port *int
	}
	payload := `{"backtest": "test", "manual": true}`
	rsp := test_helper.Request(test.T(), http.MethodPost, "/backtest/api/sessions", payload)
	if !test.Equal(http.StatusCreated, rsp.StatusCode) {
		rspData, _ := io.ReadAll(rsp.Body)
		test.Failf("Failed to create session: %s", string(rspData))
	}
	data := &SessionResponse{}
	err := json.NewDecoder(rsp.Body).Decode(data)
	if err != nil {
		test.Failf("Failed to decode response: %s", err.Error())
	}
	condition := func() (bool, error) {
		rsp := test_helper.Request(test.T(), http.MethodGet, "/backtest/api/sessions/"+data.ID, nil)
		if rsp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("failed to get session: %d", rsp.StatusCode)
		}
		data := &SessionResponse{}
		err := json.NewDecoder(rsp.Body).Decode(data)
		if err != nil {
			return false, fmt.Errorf("failed to decode response: %s", err.Error())
		}
		if data.Statuses[0].Status == "FAILED" {
			return false, fmt.Errorf("session failed")
		}
		if data.Port != nil {
			return true, nil
		}
		return false, nil
	}
	test.NoError(test_helper.WaitUntilCondition(test.T(), condition, time.Second*30))

	rsp = test_helper.Request(test.T(), http.MethodGet, "/backtest/api/sessions/"+data.ID, nil)
	test.Equal(http.StatusOK, rsp.StatusCode)
	data = &SessionResponse{}
	err = json.NewDecoder(rsp.Body).Decode(data)
	test.NoError(err)

	test.T().Logf("Connecting to port %d", *data.Port)
	socket, err := reqSocket.NewSocket()
	test.NoError(err)
	defer socket.Close()
	err = socket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", *data.Port))
	test.NoError(err)
	test.NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second*5))
	test.NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second*5))

	test.T().Log("Sending new_execution")
	new_execution_req := backtest_pb.NewExecutionRequest{
		Algorithm: &service_pb.Algorithm{
			FilePath: "/algo.py",
			Functions: []*service_pb.Algorithm_Function{
				{
					Name:              "handle_data",
					ParallelExecution: true,
				},
			},
		},
	}
	new_execution_rsp := backtest_pb.NewExecutionResponse{}
	test_helper.SocketRequest(test.T(), socket, "new_execution", &new_execution_req, &new_execution_rsp)
	conf_execution_req := backtest_pb.ConfigureExecutionRequest{
		Execution: new_execution_rsp.Id,
		StartDate: new_execution_rsp.StartDate,
		EndDate:   new_execution_rsp.EndDate,
		Symbols:   new_execution_rsp.Symbols,
	}
	conf_execution_rsp := service_pb.ConfigureExecutionRequest{}
	test_helper.SocketRequest(test.T(), socket, "configure_execution", &conf_execution_req, &conf_execution_rsp)

	test.Require().NotZero(conf_execution_rsp.BrokerPort, "Broker port is zero")
	workerSocket, err := repSocket.NewSocket()
	test.NoError(err)
	err = workerSocket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", conf_execution_rsp.BrokerPort))
	test.Require().NoError(err, fmt.Sprintf("Failed to connect to broker port %d", conf_execution_rsp.BrokerPort))

	type WorkerRequest struct {
		Execution string
		Timestamp time.Time
		Symbol    string
		Portfolio struct {
			Cash      float64
			Value     float64
			Positions []struct {
				Symbol string
			}
		}
	}
	go test_helper.SocketReplier(test.T(), workerSocket, func(data interface{}) (proto.Message, error) {
		time.Sleep(time.Second / 8) // Simulate work
		return nil, nil
	})

	test.T().Log("Sending run_execution")
	run_execution_req := backtest_pb.RunExecutionRequest{}
	test_helper.SocketRequest(test.T(), socket, "run_execution", &run_execution_req, nil)
	time.Sleep(time.Second)
	for {
		portfolio_rsp := backtest_pb.GetPortfolioResponse{}
		test_helper.SocketRequest(test.T(), socket, "current_portfolio", nil, &portfolio_rsp)
		if portfolio_rsp.Timestamp == nil {
			break
		}
		time.Sleep(time.Second / 4)
	}
	test.T().Log("Sending stop")

	test_helper.SocketRequest(test.T(), socket, "stop", nil, nil)
	time.Sleep(time.Second * 5)
	workerSocket.Close()
}
