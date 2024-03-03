package service

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
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type ServiceModuleTest struct {
	suite.Suite

	app *fx.App
}

func TestModuleService(t *testing.T) {
	workerImage := os.Getenv("WORKER_IMAGE")
	if workerImage == "" {
		t.Skip("worker image not set")
	}
	backtestImage := os.Getenv("BACKTEST_IMAGE")
	if backtestImage == "" {
		t.Skip("backtest image not set")
	}
	suite.Run(t, new(ServiceModuleTest))
}

func (test *ServiceModuleTest) SetupTest() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
		NATS:     true,
		Loki:     true,
	})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.NoError(err)
	test.app = fx.New(
		fx.Provide(
			func() (nats.JetStreamContext, error) {
				return stream.NewJetstream()
			},
			func() *pgxpool.Pool {
				return pool
			},
			func() *gin.Engine {
				return h.NewEngine()
			},
		),
		fx.Invoke(
			h.NewLifeCycleRouter,
		),
		stream.OrchestrationLifecycle,
		Module,
	)
	test.NoError(test.app.Start(context.TODO()))
}

func (test *ServiceModuleTest) TearDownTest() {
	helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()))
}

func (test *ServiceModuleTest) TestCreateService() {
	type ServiceResponse struct {
		Name     string
		Type     string
		Statuses []struct {
			Status string
		}
	}
	type TestCase struct {
		ServiceName  string
		ServiceImage string
		ExpectedType string
	}
	testCases := []TestCase{
		{
			ServiceName:  "backtest",
			ServiceImage: os.Getenv("BACKTEST_IMAGE"),
			ExpectedType: "backtest",
		},
		{
			ServiceName:  "worker",
			ServiceImage: os.Getenv("WORKER_IMAGE"),
			ExpectedType: "worker",
		},
	}
	for _, testcase := range testCases {
		test.Run(testcase.ServiceName, func() {
			payload := `{"name": "` + testcase.ServiceName + `", "image": "` + testcase.ServiceImage + `"}`
			rsp := helper.Request(test.T(), http.MethodPost, "/service/api/services", payload)
			if !test.Equal(http.StatusCreated, rsp.StatusCode) {
				rspData, _ := io.ReadAll(rsp.Body)
				test.Failf("Failed to create service: %s", string(rspData))
			}
			condition := func() (bool, error) {
				rsp = helper.Request(test.T(), http.MethodGet, "/service/api/services/"+testcase.ServiceName, nil)
				if rsp.StatusCode != http.StatusOK {
					return false, fmt.Errorf("failed to get service: %d", rsp.StatusCode)
				}
				data := &ServiceResponse{}
				err := json.NewDecoder(rsp.Body).Decode(data)
				if err != nil {
					return false, fmt.Errorf("failed to decode response: %s", err.Error())
				}
				if data.Statuses[0].Status != "READY" {
					return false, nil
				}
				return true, nil
			}
			err := helper.WaitUntilCondition(test.T(), condition, time.Second*30)
			test.NoError(err)
			rsp = helper.Request(test.T(), http.MethodGet, "/service/api/services/"+testcase.ServiceName, nil)
			if !test.Equal(http.StatusOK, rsp.StatusCode) {
				rspData, _ := io.ReadAll(rsp.Body)
				test.Failf("Failed to get service: %s", string(rspData))
			}
			data := &ServiceResponse{}
			err = json.NewDecoder(rsp.Body).Decode(data)
			if err != nil {
				test.Failf("Failed to decode response: %w", err.Error())
				return
			}
			test.Equal(testcase.ServiceName, data.Name)
			test.Equal(testcase.ExpectedType, data.Type)
		})
	}
}
