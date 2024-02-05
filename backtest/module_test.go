package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	repSocket "go.nanomsg.org/mangos/v3/protocol/rep"
	reqSocket "go.nanomsg.org/mangos/v3/protocol/req"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

type BacktestModuleTest struct {
	suite.Suite
	app *fx.App

	backtestServiceName string
	workerServiceName   string
	backtestName        string
}

func TestModuleBacktest(t *testing.T) {
	workerImage := os.Getenv("WORKER_IMAGE")
	if workerImage == "" {
		t.Skip("worker image not set")
	}
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
	})

	log := zaptest.NewLogger(test.T(), zaptest.Level(zap.DebugLevel))

	st, err := stream.NewJetstream()
	test.Require().NoError(err)

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.Require().NoError(err)

	g := h.NewEngine()

	store, err := storage.NewMinioStorage()
	test.Require().NoError(err)

	test.app = fx.New(
		fx.Provide(
			func() *zap.Logger {
				return log
			},
			func() nats.JetStreamContext {
				return st
			},
			func() *pgxpool.Pool {
				return pool
			},
			func() *gin.Engine {
				return g
			},
			func() storage.BlobStorage {
				return store
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
	createService := func(name, image string) error {
		helper.Request(test.T(), http.MethodDelete, "/service/api/services/"+name, nil)
		payload := `{"name":"` + name + `","image":"` + image + `"}`
		rsp := helper.Request(test.T(), http.MethodPost, "/service/api/services", payload)
		if !test.Equal(http.StatusCreated, rsp.StatusCode) {
			rspData, _ := io.ReadAll(rsp.Body)
			return fmt.Errorf("failed to create service: %d %s", rsp.StatusCode, string(rspData))
		}
		condition := func() (bool, error) {
			type ServiceResponse struct {
				Name     string
				Type     string
				Statuses []struct {
					Status string
				}
			}

			rsp = helper.Request(test.T(), http.MethodGet, "/service/api/services/"+name, nil)
			if rsp.StatusCode != http.StatusOK {
				return false, fmt.Errorf("failed to get service: %d", rsp.StatusCode)
			}
			data := &ServiceResponse{}
			err := json.NewDecoder(rsp.Body).Decode(data)
			if err != nil {
				return false, fmt.Errorf("failed to decode response: %s", err.Error())
			}
			if data.Statuses[0].Status == "ERROR" {
				return false, fmt.Errorf("service failed")
			}

			if data.Statuses[0].Status != "READY" {
				return false, nil
			}
			return true, nil
		}
		return helper.WaitUntilCondition(test.T(), condition, time.Second*30)
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	createServices, _ := errgroup.WithContext(ctx)
	createServices.Go(func() error {
		return createService("backtest", os.Getenv("BACKTEST_IMAGE"))
	})
	createServices.Go(func() error {
		return createService("worker", os.Getenv("WORKER_IMAGE"))
	})
	err = createServices.Wait()
	test.Require().NoError(err)

	test.backtestServiceName = "backtest"
	test.workerServiceName = "worker"

	payload := `{"name":"test","backtest_service":"` + test.backtestServiceName + `", "worker_service":"` + test.workerServiceName + `","symbols":["AAPL"],"calendar": "XNYS", "start":"2020-01-01T00:00:00Z","end":"2020-01-31T00:00:00Z"}`
	rsp := helper.Request(test.T(), http.MethodPost, "/backtest/api/backtests", payload)
	if !test.Equal(http.StatusCreated, rsp.StatusCode) {
		rspData, _ := io.ReadAll(rsp.Body)
		test.Failf("Failed to create backtest: %s", string(rspData))
	}
	condition := func() (bool, error) {
		type BacktestResponse struct {
			Statuses []struct {
				Status string
			}
		}
		rsp := helper.Request(test.T(), http.MethodGet, "/backtest/api/backtests/test", nil)
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

	test.backtestName = "test"
}

func (test *BacktestModuleTest) TearDownSuite() {
	helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()))
}

func (test *BacktestModuleTest) TestRunBacktestAutomatic() {
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
		if data.Statuses[0].Status == "COMPLETED" {
			return true, nil
		}
		return false, nil
	}
	test.NoError(helper.WaitUntilCondition(test.T(), condition, time.Second*30))
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

	socket, err := reqSocket.NewSocket()
	test.NoError(err)
	defer socket.Close()
	err = socket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", *data.Port))
	test.NoError(err)

	execution := new(entity.Execution)
	test.NoError(helper.SocketRequest(test.T(), socket, "new_execution", nil, execution))

	workerSocket, err := repSocket.NewSocket()
	test.NoError(err)
	err = workerSocket.Dial(fmt.Sprintf("tcp://127.0.0.1:%d", *execution.Port))
	test.NoError(err)
	go helper.SocketReplier(test.T(), workerSocket, func(data interface{}) (interface{}, error) {
		return nil, nil
	})
	test.NoError(helper.SocketRequest(test.T(), socket, "run_execution", nil, nil))
	test.NoError(helper.SocketRequest(test.T(), socket, "stop", nil, nil))
	time.Sleep(time.Second * 5)
	workerSocket.Close()
}
