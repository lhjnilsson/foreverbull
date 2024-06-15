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
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/finance"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service"
	serviceAPI "github.com/lhjnilsson/foreverbull/service/api"
	serviceEntity "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/mitchellh/mapstructure"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	repSocket "go.nanomsg.org/mangos/v3/protocol/rep"
	reqSocket "go.nanomsg.org/mangos/v3/protocol/req"
	"go.uber.org/fx"
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
	helper.SetupEnvironment(test.T(), &helper.Containers{
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
	rsp := helper.Request(test.T(), http.MethodPost, "/backtest/api/ingestion", payload)
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
		rsp := helper.Request(test.T(), http.MethodGet, "/backtest/api/ingestion", nil)
		if rsp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("failed to get backtest: %d", rsp.StatusCode)
		}
		data := &BacktestResponse{}
		err := json.NewDecoder(rsp.Body).Decode(data)
		if err != nil {
			return false, fmt.Errorf("failed to decode response: %s", err.Error())
		}
		if data.Statuses[0].Status == "ERROR" {
			return false, fmt.Errorf("backtest failed")
		}

		if data.Statuses[0].Status == "READY" {
			return true, nil
		}
		return false, nil
	}
	test.Require().NoError(helper.WaitUntilCondition(test.T(), condition, time.Second*30))

	payload = `{"name":"test","symbols":["AAPL"],"calendar": "XNYS", "start":"2020-01-01T00:00:00Z","end":"2020-01-31T00:00:00Z"}`
	rsp = helper.Request(test.T(), http.MethodPost, "/backtest/api/backtests", payload)
	if !test.Equal(http.StatusCreated, rsp.StatusCode) {
		rspData, _ := io.ReadAll(rsp.Body)
		test.Failf("Failed to create backtest: %s", string(rspData))
	}
	test.backtestName = "test"
}

func (test *BacktestModuleTest) TearDownSuite() {
	helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()))
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
			rsp := helper.Request(test.T(), http.MethodPost, "/backtest/api/sessions", payload)
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
				rsp := helper.Request(test.T(), http.MethodGet, "/backtest/api/sessions/"+data.ID, nil)
				if rsp.StatusCode != http.StatusOK {
					return false, fmt.Errorf("failed to get session: %d", rsp.StatusCode)
				}
				data := &SessionResponse{}
				err := json.NewDecoder(rsp.Body).Decode(data)
				if err != nil {
					return false, fmt.Errorf("failed to decode response: %s", err.Error())
				}
				if data.Statuses[0].Status == "COMPLETED" {
					return true, nil
				}
				return false, nil
			}
			test.NoError(helper.WaitUntilCondition(test.T(), condition, time.Second*30))
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
	rsp := helper.Request(test.T(), http.MethodPost, "/backtest/api/sessions", payload)
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
		rsp := helper.Request(test.T(), http.MethodGet, "/backtest/api/sessions/"+data.ID, nil)
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
	test.NoError(helper.WaitUntilCondition(test.T(), condition, time.Second*30))

	rsp = helper.Request(test.T(), http.MethodGet, "/backtest/api/sessions/"+data.ID, nil)
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
	execution := new(entity.Execution)
	algorithm := []byte(`{"file_path": "/algo.py", "functions": [{"name": "handle_data", "parallel_execution": true}]}`)
	algo := &serviceEntity.Algorithm{}
	err = json.Unmarshal([]byte(algorithm), algo)
	test.NoError(err)
	test.NoError(helper.SocketRequest(test.T(), socket, "new_execution", algo, execution))

	test.T().Log("Sending configure_execution")
	instance := new(serviceEntity.Instance)
	test.NoError(helper.SocketRequest(test.T(), socket, "configure_execution", execution, instance))

	test.Require().NotNil(instance.BrokerPort)
	workerSocket, err := repSocket.NewSocket()
	test.NoError(err)
	err = workerSocket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", *instance.BrokerPort))
	test.NoError(err)

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
	go helper.SocketReplier(test.T(), workerSocket, func(data interface{}) (interface{}, error) {
		time.Sleep(time.Second / 4) // Simulate work
		wr := WorkerRequest{}
		err = mapstructure.Decode(data, &wr)
		return nil, nil
	})

	test.T().Log("Sending run_execution")
	test.NoError(helper.SocketRequest(test.T(), socket, "run_execution", nil, nil))
	time.Sleep(time.Second)
	for {
		period := &entity.Period{}
		test.Require().NoError(helper.SocketRequest(test.T(), socket, "current_period", nil, period))
		if period.Timestamp.IsZero() {
			break
		}
		time.Sleep(time.Second / 5)
	}
	test.T().Log("Sending stop")
	test.NoError(helper.SocketRequest(test.T(), socket, "stop", nil, nil))
	time.Sleep(time.Second * 5)
	workerSocket.Close()
}

func TestParse(t *testing.T) {
	payload := []byte(`{"file_path": "/algo.py", "functions": [{"name": "handle_data", "parallel_execution": true}]}`)
	algorithm := &serviceEntity.Algorithm{}
	err := json.Unmarshal(payload, algorithm)
	if err != nil {
		t.Fatal(err)
	}
}
